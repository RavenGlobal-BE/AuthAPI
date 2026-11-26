package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"log"
	"net/http"
	"raven/auth/SecureInput"
	metastrip "raven/auth/SecureInput/Image"
	"time"
	"uuid"

	_ "image/jpeg"
	_ "image/png"

	"github.com/gin-gonic/gin"
)

const maxUpload = 4 << 20

func (a *App) uploadPhoto(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUpload+1<<20)
	ctx := c.Request.Context()

	raw, exists := c.Get("user_id")
	userID, ok := raw.(int64)
	if !exists || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return // was missing: the type assertion below would have panicked
	}

	user := a.ur.GetAccountById(userID)
	if user == nil { // adjust if GetAccountById returns (Account, error)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid file"})
		return
	}
	if fh.Size > maxUpload {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
		return
	}

	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	if len(data) > maxUpload {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
		return
	}

	fileType := SecureInput.GetTypeBySignature(data)
	if fileType == SecureInput.NONE {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type"})
		return
	}

	var stripErr error
	switch fileType {
	case SecureInput.JPG:
		data, stripErr = metastrip.JPEG(data)
	case SecureInput.PNG:
		data, stripErr = metastrip.PNG(data)
	case SecureInput.HEIC:
		data, stripErr = metastrip.HEIC(data)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type"})
		return
	}
	if stripErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image"})
		return
	}

	if fileType == SecureInput.JPG || fileType == SecureInput.PNG {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width*cfg.Height > 50_000_000 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image"})
			return
		}
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image"})
			return
		}
	}

	ext := map[SecureInput.FileType]string{
		SecureInput.JPG: ".jpg", SecureInput.PNG: ".png", SecureInput.HEIC: ".heic", // .heic was missing
	}[fileType]
	ct := map[SecureInput.FileType]string{
		SecureInput.JPG: "image/jpeg", SecureInput.PNG: "image/png", SecureInput.HEIC: "image/heic",
	}[fileType]
	name := uuid.New().String() + ext

	if err := SecureInput.UploadImageToCDN(ctx, name, ct, data); err != nil {
		log.Printf("cdn upload: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "upload failed"})
		return
	}

	old, err := a.ur.UpdatePhoto(ctx, name, userID)
	if err != nil {
		log.Printf("update photo: %v", err)
		if err := SecureInput.RemoveImageFromCDN(ctx, name); err != nil {
			log.Printf("cdn cleanup: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update photo entry"})
		return
	}

	if old != nil && *old != "" {
		if err := SecureInput.RemoveImageFromCDN(ctx, *old); err != nil {
			log.Printf("cdn remove: %v", err)
		}
	}

	c.JSON(http.StatusCreated, gin.H{"url": "cdn-p1.raven.co.com/auth/profile/" + name, "key": name})
}

var ErrUserNotFound = errors.New("user not found")

func (a *App) removePhoto(c *gin.Context) {
	raw, exists := c.Get("user_id")
	userID, ok := raw.(int64)
	if !exists || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	old, err := a.ur.RemovePhoto(c.Request.Context(), userID)
	if errors.Is(err, ErrUserNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to remove photo"})
		return
	}
	if old == nil || *old == "" {
		c.JSON(http.StatusOK, gin.H{"success": true}) // nothing to remove
		return
	}

	go func(file string) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("CDN cleanup panic: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := SecureInput.RemoveImageFromCDN(ctx, file); err != nil {
			log.Printf("CDN cleanup failed for user %d (%s): %v", userID, file, err)
		}
	}(*old)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

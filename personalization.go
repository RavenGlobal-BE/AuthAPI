package main

import (
	"bytes"
	"context"
	"image"
	"io"
	"log"
	"net/http"
	"raven/auth/SecureInput"
	"time"
	"uuid"

	"github.com/gin-gonic/gin"

	_ "image/jpeg"
	_ "image/png"
)

const maxUpload = 8 << 20

func (a *App) uploadPhoto(c *gin.Context) {
	userID, exists := c.Get("user_id")

	if exists != true {
		c.JSON(400, gin.H{"error": "Invalid token"})
	}

	user := a.ur.GetAccountById(userID.(int64))

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid file"})
		return
	}

	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	defer f.Close()

	// Read the file into memory, with a hard limit.
	data, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	if len(data) > maxUpload {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
		return
	}

	// Signature check
	fileType := SecureInput.GetTypeBySignature(data)
	if fileType == SecureInput.NONE {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file type"})
		return
	}

	// Decode check (JPG and PNG). Read the dimensions first, then decode.
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

	ext := map[SecureInput.FileType]string{SecureInput.JPG: ".jpg", SecureInput.PNG: ".png"}[fileType]
	ct := map[SecureInput.FileType]string{
		SecureInput.JPG: "image/jpeg", SecureInput.PNG: "image/png", SecureInput.HEIC: "image/heic",
	}[fileType]
	name := uuid.New().String() + ext

	// THIS IS COMING SOON FOR CUSTOM HOSTING
	//dst := filepath.Join("./files", name)

	// //0o600 means only the server's current user effectively owns the file. No one can edit or delete it but the current user.
	//if err := os.WriteFile(dst, data, 0o600); err != nil {
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save file"})
	//	return
	//}

	if err := SecureInput.UploadImageToCDN(c.Request.Context(), name, ct, data); err != nil {
		log.Printf("cdn upload: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "upload failed"})
		return
	}

	if &user.ProfilePicture != nil {
		if err := SecureInput.RemoveImageFromCDN(c.Request.Context(), *user.ProfilePicture); err != nil {
			log.Printf("cdn remove: %v", err)
		}
	}

	updateError := a.ur.UpdatePhoto(context.Background(), name, userID.(int64))
	if updateError != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update photo entry"})
	}

	c.JSON(http.StatusCreated, gin.H{"url": "cdn-p1.raven.co.com/auth/profile/" + name})
}

func (a *App) removePhoto(c *gin.Context) {
	raw, exists := c.Get("user_id")
	userID, ok := raw.(int64)
	if !exists || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// Grab the current photo URL BEFORE clearing it.
	acc := a.ur.GetAccountById(userID)
	if acc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
		return
	}
	profile := acc.ProfilePicture
	if *profile == "" {
		c.JSON(http.StatusOK, gin.H{"success": true}) // nothing to remove
		return
	}

	if err := a.ur.RemovePhoto(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to remove photo"})
		return
	}

	// CDN cleanup in the background: don't touch c, use a detached context.
	go func(url string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := SecureInput.RemoveImageFromCDN(ctx, url); err != nil {
			log.Printf("CDN cleanup failed for user %d (%s): %v", userID, url, err)
		}
	}(*profile)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

package main

import (
	"bytes"
	"context"
	"image"
	"io"
	"log"
	"net/http"
	"raven/auth/SecureInput"
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

	updateError := a.ur.UpdatePhoto(context.Background(), name, userID.(int64))
	if updateError != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to update photo entry"})
	}

	c.JSON(http.StatusCreated, gin.H{"url": "cdn-p1.raven.co.com/auth/profile/" + name})
}

func (a *App) removePhoto(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if exists != true {
		c.JSON(400, gin.H{"error": "Invalid token"})
	}

	err := a.ur.RemovePhoto(context.Background(), userID.(int64))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to remove photo"})
	}

	c.JSON(200, gin.H{"success": true})
}

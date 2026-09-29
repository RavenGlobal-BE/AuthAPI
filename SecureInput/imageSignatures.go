package SecureInput

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type ImageFile struct {
	FileName    string
	FileContent []byte
}

type FileType int

const (
	NONE FileType = iota
	JPG
	PNG
	HEIC
)

// Returns the type based on the FileContent's signature.
func GetTypeBySignature(content []byte) FileType {
	if bytes.HasPrefix(content, []byte{0xFF, 0xD8, 0xFF}) {
		return JPG
	}

	if bytes.HasPrefix(content, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return PNG
	}

	return NONE
}

var client = &http.Client{Timeout: 30 * time.Second}

func UploadImageToCDN(ctx context.Context, fileName, contentType string, data []byte) error {
	host := os.Getenv("BUNNY_STORAGE_HOST") // e.g. storage.bunnycdn.com
	zone := os.Getenv("BUNNY_STORAGE_ZONE")
	key := os.Getenv("BUNNY_STORAGE_PASSWORD")

	url := fmt.Sprintf("https://%s/%s/%s", host, zone, fileName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	req.Header.Set("AccessKey", key)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Checksum", strings.ToUpper(hex.EncodeToString(sum[:])))

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("bunny upload failed: %s: %s", res.Status, body)
	}
	return nil
}

func RemoveImageFromCDN(ctx context.Context, fileName string) error {
	host := os.Getenv("BUNNY_STORAGE_HOST")
	zone := os.Getenv("BUNNY_STORAGE_ZONE")
	key := os.Getenv("BUNNY_STORAGE_PASSWORD")

	url := fmt.Sprintf("https://%s/%s/%s", host, zone, fileName)

	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	req.Header.Set("AccessKey", key)

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("image deletion failed")
	}
	return nil
}

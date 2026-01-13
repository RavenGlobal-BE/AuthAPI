package authorization

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes), err
}

func CheckPasswordHash(password string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateToken() (*string, error) {
	//Generates a random 32-byte token
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tokenBytes)

	// Hash token with SHA-256
	hash := sha256.Sum256([]byte(token))
	hashedToken := hex.EncodeToString(hash[:])

	//Adds 180 days to the current time for expiration
	expiresAt := time.Now().AddDate(0, 0, 180).Format("2006-01-02 15:04:05")

	fmt.Println("token:", token)
	fmt.Println("hashedToken:", hashedToken)
	fmt.Println("expiresAt:", expiresAt)

	return &token, nil
}

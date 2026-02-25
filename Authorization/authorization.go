package authorization

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 1
	argonMemory  = 16 * 1024 // 16 MB
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return "$argon2id$" + b64Salt + "$" + b64Hash, nil
}

func CheckPasswordHash(password string, encodedHash string) bool {
	if len(encodedHash) < 10 || !strings.HasPrefix(encodedHash, "$argon2id$") {
		return false
	}
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 4 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	testHash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, uint32(len(hash)))
	return subtle.ConstantTimeCompare(hash, testHash) == 1
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

	return &hashedToken, nil
}
func CreateKeyPairs() (string, string, error) {
	token, err := GenerateToken()
	if err != nil {
		return "", "", err
	}

	refresh, err := GenerateToken()
	if err != nil {
		return "", "", err
	}

	return *token, *refresh, nil
}

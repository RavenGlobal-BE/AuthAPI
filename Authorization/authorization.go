package authorization

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Optimized for our OVH Container.
// Specs: 8 vCPU, 24GB of RAM -> Unlike bcrypt, argon2 is memory hungry.
const (
	argonTime    = 3
	argonMemory  = 48 * 1024 // 48 MB
	argonThreads = 2
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

func HashLegacyPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func SHA256Hash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func CheckPasswordHash(password string, encodedHash string) bool { //This is used to test the argon2id hash
	if strings.HasPrefix(encodedHash, "$2b$") || strings.HasPrefix(encodedHash, "$2a$") || strings.HasPrefix(encodedHash, "$2y$") {
		err := bcrypt.CompareHashAndPassword([]byte(encodedHash), []byte(password))
		return err == nil
	}

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

// Gets location information from IP
func GetIPLocation(ip string) (map[string]interface{}, error) {
	if ip == "::1" || ip == "127.0.0.1" {
		return map[string]interface{}{
			"country": "US",
			"city":    "San Francisco",
			"lat":     37.7749,
			"lon":     -122.4194,
			"carrier": "Raven Particle",
		}, nil
	}

	url := "http://ip-api.com/json/" + ip

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("error decoding json: %w", err)
	}

	var formattedData = map[string]interface{}{
		"country": data["countryCode"],
		"city":    data["regionName"],
		"lat":     data["lat"],
		"lon":     data["lon"],
		"carrier": data["isp"],
	}
	return formattedData, nil
}

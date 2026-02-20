package authorization

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTPayload struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// Both getPrivateKey() & getPublicKey() use the Ed25519 algorithm to create the keys
func getPrivateKey() (ed25519.PrivateKey, error) {
	keyStr := os.Getenv("JWT_PRIVATE_KEY")
	if keyStr == "" {
		return nil, errors.New("JWT_PRIVATE_KEY .env variable not set. Set one IMMEDIATELY")
	}
	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, err
	}
	return ed25519.PrivateKey(keyBytes), nil
}

func getPublicKey() (ed25519.PublicKey, error) {
	keyStr := os.Getenv("JWT_PUBLIC_KEY")
	if keyStr == "" {
		return nil, errors.New("JWT_PUBLIC_KEY .env variable not set. Set one IMMEDIATELY")
	}
	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(keyBytes), nil
}

func GenerateAccessToken(userID int32, email, firstName string) (string, error) {
	expirationTime := time.Now().Add(15 * time.Minute)

	tokenID, err := GenerateToken()
	if err != nil {
		return "", err
	}

	payload := &JWTPayload{
		Email:     email,
		FirstName: firstName,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        *tokenID,
			Issuer:    "https://auth.raven.co.com",
			Subject:   fmt.Sprintf("%d", userID), //Over wie deze token gaat (vervangt UserID)
			Audience:  jwt.ClaimStrings{"Raven-Original"},
		},
	}

	privateKey, err := getPrivateKey()
	if err != nil {
		fmt.Println(err)

		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, payload)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		fmt.Println(err)

		return "", err
	}

	return tokenString, nil
}

// Creates a refresh token that lasts 14 days
func GenerateRefreshToken(userID int32, email string) (string, error) {
	expirationTime := time.Now().Add(14 * 24 * time.Hour)

	tokenID, err := GenerateToken()
	if err != nil {
		return "", err
	}

	payload := &JWTPayload{
		Email:     email,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        *tokenID,
			Issuer:    "https://auth.raven.co.com",
			Subject:   fmt.Sprintf("%d", userID), //Over wie deze token gaat (vervangt UserID)
			Audience:  jwt.ClaimStrings{"Raven-Originals"},
		},
	}

	privateKey, err := getPrivateKey()
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, payload)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return tokenString, nil
}

// Parses and validates the JWT token
func ValidateToken(tokenString string) (*JWTPayload, error) {
	claims := &JWTPayload{}

	publicKey, err := getPublicKey()
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, errors.New("unexpected signing method")
		}

		return publicKey, nil
	})

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

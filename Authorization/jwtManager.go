package authorization

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	logger "raven/auth/Logging"

	"github.com/golang-jwt/jwt/v5"
)

type JWTPayload struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Country   string `json:"country"`
	TokenType string `json:"token_type"`
	Nonce     string `json:"nonce,omitempty"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

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

func GenerateJWTToken(userID int64, email, firstName, lastName, access string, expiration time.Time, nonce, sessionID, country, audience string) (string, error) {
	tokenID, err := GenerateToken()
	if err != nil {
		return "", err
	}

	payload := &JWTPayload{
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		TokenType: access, // "access" of "refresh"
		Country:   country,
		Nonce:     nonce,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiration),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        *tokenID,
			Issuer:    os.Getenv("JWT_ISSUER"),
			Subject:   fmt.Sprintf("%d", userID), //Over wie deze token gaat (vervangt UserID)
			Audience:  jwt.ClaimStrings{"Raven-Original"},
		},
	}

	privateKey, err := getPrivateKey()
	if err != nil {
		logger.Log(err.Error(), logger.Error)
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, payload)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		logger.Log(err.Error(), logger.Error)
		return "", err
	}

	return tokenString, nil
}

// Parses and validates the JWT token
func ValidateToken(tokenString string) (*JWTPayload, error) {
	claims := &JWTPayload{}

	publicKey, err := getPublicKey()
	if err != nil {
		logger.Log(err.Error(), logger.Debug)
		return nil, err
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok { // Verifying the signing method is correct
			return nil, errors.New("unexpected signing method")
		}

		if claims.Issuer != os.Getenv("JWT_ISSUER") { // Verifying whether the issuer truely comes from Raven.
			return nil, errors.New("invalid token issuer")
		}

		if claims.Audience == nil || claims.Audience[0] != "Raven-Original" { // Check whether the token is really meant for "Raven Original" services.
			return nil, errors.New("invalid token audience")
		}

		if claims.ExpiresAt.Before(time.Now()) { //Checks whether the token is expired
			return nil, errors.New("token has expired")
		}

		if claims.TokenType != "access" && claims.TokenType != "refresh" { //Checks whether it's a valid type of token
			return nil, errors.New("invalid token type")
		}

		return publicKey, nil
	})

	if err != nil {
		logger.Log(err.Error(), logger.Debug)
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

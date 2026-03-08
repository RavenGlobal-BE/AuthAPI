package main

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/gin-gonic/gin"
)

func JWTKeys(c *gin.Context) {
	publicKey := os.Getenv("JWT_PUBLIC_KEY")
	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKey)
	var x string

	if err == nil && len(pubKeyBytes) == 32 {
		x = base64.RawURLEncoding.EncodeToString(pubKeyBytes)
	} else {
		x = ""
	}

	c.JSON(200, gin.H{"keys": []gin.H{
		{
			"kty": "OKP",
			"crv": "Ed25519",
			"x":   x,
			"alg": "EdDSA",
			"use": "sig",
			"kid": "raven_ed25519-2026-02-25",
		},
	}})
}

func OpenIDConfig(c *gin.Context) {
	c.JSON(200, gin.H{
		"issuer":                                "https://auth.raven.co.com",
		"jwks_uri":                              "https://auth.raven.co.com/.well-known/jwks.json",
		"id_token_signing_alg_values_supported": []string{"EdDSA"},
		"response_types_supported":              []string{"code"},
		"scopes_supported":                      []string{"openid", "profile", "email"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"authorization_endpoint":                "https://auth.raven.co.com/authorize",
		"token_endpoint":                        "https://auth.raven.co.com/token",
		"introspection_endpoint":                "https://auth.raven.co.com/introspect",
		"subject_types_supported":               []string{"public"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
		"claims_supported":                      []string{"sub", "email", "given_name", "family_name", "iat", "exp"},
	})
}

func (A *App) todayBG(c *gin.Context) {
	c.JSON(200, gin.H{"bg": "https://cdn.raven.co.com/authPhotos/Malta.JPG", "photoBy": "Kenza", "Location": "Marsaxlokk", "Country": "MT"})
}

func (a *App) clientInfo(c *gin.Context) {
	clientID := c.Query("client_id")

	client := a.cr.GetClientByID(context.Background(), clientID)
	if client == nil {
		c.JSON(401, gin.H{"error": "Invalid client"})
		return
	}

	imageURL := ""
	if client.CompanyIcon != nil {
		imageURL = *client.CompanyIcon
	}
	c.JSON(200, gin.H{"imageEnabled": client.CompanyIcon != nil, "imageURL": imageURL})
}

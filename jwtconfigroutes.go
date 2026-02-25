package main

import (
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
		"issuer":                                "https://auth.raven.com",
		"jwks_uri":                              "https://auth.raven.com/.well-known/jwks.json",
		"id_token_signing_alg_values_supported": []string{"EdDSA"},
	})
}

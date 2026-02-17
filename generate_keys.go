package main

import (
	"fmt"

	"crypto/ed25519"
	"encoding/base64"
)

/* ONLY RUN THIS WHEN YOU NEED TO GENERATE KEYS TO GENERATE THE PAIRS. RUN THIS ONCE AND PUT IT IN THE .ENV */

func generateKeyPair() {
	privateKey, publicKey, err := GenerateKeyPair()
	if err != nil {
		fmt.Println("Error generating keys:", err)
		return
	}

	fmt.Println("=== Ed25519 Key Pair Generated ===")
	fmt.Println("JWT_PRIVATE_KEY=" + privateKey)
	fmt.Println("JWT_PUBLIC_KEY=" + publicKey)
}

func GenerateKeyPair() (privateKey string, publicKey string, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", "", err
	}

	privateKeyB64 := base64.StdEncoding.EncodeToString(priv)
	publicKeyB64 := base64.StdEncoding.EncodeToString(pub)

	return privateKeyB64, publicKeyB64, nil
}

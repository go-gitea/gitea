// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
)

const rsaBits = 2048

// GenerateKeyPair generates a public and private keypair for signing actions by users for activitypub purposes
func GenerateKeyPair() (string, string, error) {
	priv, _ := rsa.GenerateKey(rand.Reader, rsaBits)
	privPem, err := pemBlockForPriv(priv)
	if err != nil {
		return "", "", err
	}
	pubPem, err := pemBlockForPub(&priv.PublicKey)
	if err != nil {
		return "", "", err
	}
	return privPem, pubPem, nil
}

func pemBlockForPriv(priv *rsa.PrivateKey) (string, error) {
	privBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
	return string(privBytes), nil
}

func pemBlockForPub(pub *rsa.PublicKey) (string, error) {
	pubASN1, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	pubBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})
	return string(pubBytes), nil
}

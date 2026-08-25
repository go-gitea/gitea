// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package generate

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"time"

	"gitea.dev/modules/consts"
	"gitea.dev/modules/util"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/ssh"
)

// NewInternalToken generate a new value intended to be used by INTERNAL_TOKEN.
func NewInternalToken() (string, error) {
	secretBytes := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, secretBytes)
	if err != nil {
		return "", err
	}

	secretKey := base64.RawURLEncoding.EncodeToString(secretBytes)

	now := time.Now()

	var internalToken string
	internalToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"nbf": now.Unix(),
	}).SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	return internalToken, nil
}

const defaultJwtSecretLen = 32

// DecodeJwtSecretBase64 decodes a base64 encoded jwt secret into bytes, and check its length
func DecodeJwtSecretBase64(src string) ([]byte, error) {
	encoding := base64.RawURLEncoding
	decoded := make([]byte, encoding.DecodedLen(len(src))+3)
	if n, err := encoding.Decode(decoded, []byte(src)); err != nil {
		return nil, err
	} else if n != defaultJwtSecretLen {
		return nil, fmt.Errorf("invalid base64 decoded length: %d, expects: %d", n, defaultJwtSecretLen)
	}
	return decoded[:defaultJwtSecretLen], nil
}

// NewJwtSecretWithBase64 generates a jwt secret with its base64 encoded value intended to be used for saving into config file
func NewJwtSecretWithBase64() ([]byte, string) {
	bytes := make([]byte, defaultJwtSecretLen)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err) // rand.Read never fails
	}
	return bytes, base64.RawURLEncoding.EncodeToString(bytes)
}

// NewSecretKey generate a new value intended to be used by SECRET_KEY.
func NewSecretKey() (string, error) {
	return util.CryptoRandomString(64), nil
}

type SSHKeyType string

const (
	SSHKeyRSA     SSHKeyType = "rsa"
	SSHKeyECDSA   SSHKeyType = "ecdsa"
	SSHKeyED25519 SSHKeyType = "ed25519"
)

func NewSSHKey(keyType SSHKeyType, bits int) (ssh.PublicKey, *pem.Block, error) {
	pub, priv, err := commonKeyGen(keyType, bits)
	if err != nil {
		return nil, nil, err
	}
	pemPriv, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, nil, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}

	return sshPub, pemPriv, nil
}

// commonKeyGen is an abstraction over rsa, ecdsa, and ed25519 generating functions
func commonKeyGen(keyType SSHKeyType, bits int) (crypto.PublicKey, crypto.PrivateKey, error) {
	switch keyType {
	case SSHKeyRSA:
		bits = util.IfZero(bits, consts.AsymKeyDefaultBitsRsa)
		if bits < consts.AsymKeyMinBitsRsa {
			return nil, nil, util.NewInvalidArgumentErrorf("invalid rsa bits: %d", bits)
		}
		privateKey, err := rsa.GenerateKey(rand.Reader, bits)
		if err != nil {
			return nil, nil, err
		}
		return &privateKey.PublicKey, privateKey, nil
	case SSHKeyED25519:
		return ed25519.GenerateKey(rand.Reader)
	case SSHKeyECDSA:
		bits = util.IfZero(bits, consts.AsymKeyDefaultBitsEcdsa)
		if bits < consts.AsymKeyMinBitsEC {
			return nil, nil, util.NewInvalidArgumentErrorf("invalid elliptic-curve bits: %d", bits)
		}
		curve, err := getEllipticCurve(bits)
		if err != nil {
			return nil, nil, err
		}
		privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		return &privateKey.PublicKey, privateKey, nil
	default:
		return nil, nil, util.NewInvalidArgumentErrorf("unknown key type: %s", keyType)
	}
}

func getEllipticCurve(bits int) (elliptic.Curve, error) {
	switch bits {
	case 256:
		return elliptic.P256(), nil
	case 384:
		return elliptic.P384(), nil
	case 521:
		return elliptic.P521(), nil
	default:
		return nil, util.NewInvalidArgumentErrorf("unsupported elliptic-curve bits: %d", bits)
	}
}

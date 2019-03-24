// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package generate

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"math/big"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// GetRandomString generate random string by specify chars.
func GetRandomString(n int) (string, error) {
	const lower = "abcdefghijklmnopqrstuvwxyz"
	const upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digit = "0123456789"
	const spec = "!@#$%^&*()_-+=[]'\";:/?.>,<`~"
	var h int = (n - 2) / 2
	var m int = (n) % 2
	buffer := make([]byte, n)
	maxLower := big.NewInt(int64(len(lower)))
	maxUpper := big.NewInt(int64(len(upper)))
	maxDigit := big.NewInt(int64(len(digit)))
	maxSpec := big.NewInt(int64(len(spec)))

	for i := 0; i < h; i++ {
		index, err := randomInt(maxLower)
		if err != nil {
			return "", err
		}

		buffer[i] = lower[index]
	}
	for i := h; i < 2*h+m; i++ {
		index, err := randomInt(maxUpper)
		if err != nil {
			return "", err
		}

		buffer[i] = upper[index]
	}

	index, err := randomInt(maxDigit)
	if err != nil {
		return "", err
	}
	buffer[n-2] = digit[index]

	index, err = randomInt(maxSpec)
	if err != nil {
		return "", err
	}
	buffer[n-1] = spec[index]
	return string(buffer), nil
}

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

// NewJwtSecret generate a new value intended to be used by LFS_JWT_SECRET.
func NewJwtSecret() (string, error) {
	JWTSecretBytes := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, JWTSecretBytes)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(JWTSecretBytes), nil
}

// NewSecretKey generate a new value intended to be used by SECRET_KEY.
func NewSecretKey() (string, error) {
	secretKey, err := GetRandomString(64)
	if err != nil {
		return "", err
	}

	return secretKey, nil
}

func randomInt(max *big.Int) (int, error) {
	rand, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}

	return int(rand.Int64()), nil
}

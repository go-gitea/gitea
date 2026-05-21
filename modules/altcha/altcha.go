// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package altcha

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
)

type Challenge struct {
	Algorithm string `json:"algorithm"`
	Challenge string `json:"challenge"`
	Salt      string `json:"salt"`
	Signature string `json:"signature"`
	MaxNumber int    `json:"maxnumber,omitempty"`
}

type Payload struct {
	Algorithm string `json:"algorithm"`
	Challenge string `json:"challenge"`
	Number    int    `json:"number"`
	Salt      string `json:"salt"`
	Signature string `json:"signature"`
}

func generateSalt() (string, error) {
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateChallenge generates a new ALTCHA challenge
func GenerateChallenge(ctx context.Context) (any, error) {
	salt, err := generateSalt()
	if err != nil {
		return nil, err
	}

	maxNumber := 100000

	// We don't generate the solution number here. We generate a "number" to create the challenge.
	// Wait, the challenge is SHA256(salt + solution_number).
	b := make([]byte, 4)
	_, err = rand.Read(b)
	if err != nil {
		return nil, err
	}
<<<<<<< HEAD

=======
	
>>>>>>> 6ff278a9cc9050976922ce6adc90b9edcdc7c48b
	// Create a random number between 0 and maxNumber
	num := (int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])) % maxNumber
	if num < 0 {
		num = -num
	}

	// Calculate challenge = SHA256(salt + num)
	hash := sha256.Sum256(fmt.Appendf(nil, "%s%d", salt, num))
	challengeStr := hex.EncodeToString(hash[:])

	// Calculate signature = HMAC-SHA256(challenge, secret)
	mac := hmac.New(sha256.New, []byte(setting.Service.AltchaSecret))
	mac.Write([]byte(challengeStr))
	signature := hex.EncodeToString(mac.Sum(nil))

	return Challenge{
		Algorithm: "SHA-256",
		Challenge: challengeStr,
		Salt:      salt,
		Signature: signature,
		MaxNumber: maxNumber,
	}, nil
}

// Verify verifies the ALTCHA payload
func Verify(ctx context.Context, payloadStr string) (bool, error) {
	if payloadStr == "" {
		return false, errors.New("empty payload")
	}

	decoded, err := base64.StdEncoding.DecodeString(payloadStr)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(payloadStr)
		if err != nil {
			return false, fmt.Errorf("decode payload failed: %w", err)
		}
	}

	var payload Payload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return false, fmt.Errorf("unmarshal payload failed: %w", err)
	}

	// 1. Verify signature
	mac := hmac.New(sha256.New, []byte(setting.Service.AltchaSecret))
	mac.Write([]byte(payload.Challenge))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if payload.Signature != expectedSignature {
		return false, errors.New("invalid signature")
	}

	// 2. Verify challenge computation
	hash := sha256.Sum256(fmt.Appendf(nil, "%s%d", payload.Salt, payload.Number))
	expectedChallenge := hex.EncodeToString(hash[:])

	if payload.Challenge != expectedChallenge {
		return false, errors.New("invalid challenge")
	}

	return true, nil
}

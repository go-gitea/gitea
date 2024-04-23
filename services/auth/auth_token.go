// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// Based on https://paragonie.com/blog/2015/04/secure-authentication-php-with-long-term-persistence#secure-remember-me-cookies

// The auth token consists of two parts: ID and token hash
// Every device login creates a new auth token with an individual id and hash.
// If a device uses the token to login into the instance, a fresh token gets generated which has the same id but a new hash.

var (
	ErrAuthTokenInvalidFormat = util.NewInvalidArgumentErrorf("auth token has an invalid format")
	ErrAuthTokenExpired       = util.NewInvalidArgumentErrorf("auth token has expired")
	ErrAuthTokenInvalidHash   = util.NewInvalidArgumentErrorf("auth token is invalid")
)

func CheckAuthToken(ctx context.Context, value string) (*auth_model.AuthToken, error) {
	if len(value) == 0 {
		return nil, nil
	}

	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return nil, ErrAuthTokenInvalidFormat
	}

	t, err := auth_model.GetAuthTokenByID(ctx, parts[0])
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return nil, ErrAuthTokenExpired
		}
		return nil, err
	}

	if t.ExpiresUnix < timeutil.TimeStampNow() {
		return nil, ErrAuthTokenExpired
	}

	hashedToken := sha256.Sum256([]byte(parts[1]))

	if subtle.ConstantTimeCompare([]byte(t.TokenHash), []byte(hex.EncodeToString(hashedToken[:]))) == 0 {
		// If an attacker steals a token and uses the token to create a new session the hash gets updated.
		// When the victim uses the old token the hashes don't match anymore and the victim should be notified about the compromised token.
		return nil, ErrAuthTokenInvalidHash
	}

	return t, nil
}

func RegenerateAuthToken(ctx context.Context, t *auth_model.AuthToken) (*auth_model.AuthToken, string, error) {
	token, hash, err := generateTokenAndHash()
	if err != nil {
		return nil, "", err
	}

	newToken := &auth_model.AuthToken{
		ID:          t.ID,
		TokenHash:   hash,
		UserID:      t.UserID,
		ExpiresUnix: timeutil.TimeStampNow().AddDuration(time.Duration(setting.LogInRememberDays*24) * time.Hour),
	}

	if err := auth_model.UpdateAuthTokenByID(ctx, newToken); err != nil {
		return nil, "", err
	}

	return newToken, token, nil
}

func CreateAuthTokenForUserID(ctx context.Context, userID int64) (*auth_model.AuthToken, string, error) {
	t := &auth_model.AuthToken{
		UserID:      userID,
		ExpiresUnix: timeutil.TimeStampNow().AddDuration(time.Duration(setting.LogInRememberDays*24) * time.Hour),
	}

	var err error
	t.ID, err = util.CryptoRandomString(10)
	if err != nil {
		return nil, "", err
	}

	token, hash, err := generateTokenAndHash()
	if err != nil {
		return nil, "", err
	}

	t.TokenHash = hash

	if err := auth_model.InsertAuthToken(ctx, t); err != nil {
		return nil, "", err
	}

	return t, token, nil
}

func generateTokenAndHash() (string, string, error) {
	buf, err := util.CryptoRandomBytes(32)
	if err != nil {
		return "", "", err
	}

	token := hex.EncodeToString(buf)

	hashedToken := sha256.Sum256([]byte(token))

	return token, hex.EncodeToString(hashedToken[:]), nil
}

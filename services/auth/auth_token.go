// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
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
		return nil, nil //nolint:nilnil // the auth method is not applicable
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

	if !util.CryptoConstTimeEqual(t.TokenHash, hex.EncodeToString(hashedToken[:])) {
		// If an attacker steals a token and uses the token to create a new session the hash gets updated.
		// When the victim uses the old token the hashes don't match anymore and the victim should be notified about the compromised token.
		// Revoke the token so the attacker's rotated token (which shares this ID) can no longer be used.
		if err := auth_model.DeleteAuthTokenByID(ctx, t.ID); err != nil {
			return nil, err
		}
		return nil, ErrAuthTokenInvalidHash
	}

	return t, nil
}

// CheckedAuthTokens is the result of validating a remember-me cookie holding several tokens.
type CheckedAuthTokens struct {
	Valid       []*auth_model.AuthToken
	Compromised bool
}

// CheckAuthTokens validates a comma separated list of tokens, keeping the usable ones.
// A single-token value written before multi-account support parses as a one element list.
func CheckAuthTokens(ctx context.Context, value string) (CheckedAuthTokens, error) {
	var ret CheckedAuthTokens
	if value == "" {
		return ret, nil
	}
	for part := range strings.SplitSeq(value, ",") {
		t, err := CheckAuthToken(ctx, part)
		switch {
		case errors.Is(err, ErrAuthTokenInvalidHash):
			ret.Compromised = true // the row is already revoked, but the other accounts stay usable
		case errors.Is(err, ErrAuthTokenInvalidFormat), errors.Is(err, ErrAuthTokenExpired):
		case err != nil:
			return ret, err
		case t != nil:
			ret.Valid = append(ret.Valid, t)
		}
	}
	return ret, nil
}

func RegenerateAuthToken(ctx context.Context, t *auth_model.AuthToken) (*auth_model.AuthToken, string, error) {
	token, hash := generateTokenAndHash()

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

	t.ID = util.CryptoRandomString(10)

	token, hash := generateTokenAndHash()

	t.TokenHash = hash

	if err := auth_model.InsertAuthToken(ctx, t); err != nil {
		return nil, "", err
	}

	return t, token, nil
}

func generateTokenAndHash() (string, string) {
	buf := util.CryptoRandomBytes(32)

	token := hex.EncodeToString(buf)

	hashedToken := sha256.Sum256([]byte(token))

	return token, hex.EncodeToString(hashedToken[:])
}

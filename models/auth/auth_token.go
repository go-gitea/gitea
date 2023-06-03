// Copyright 2023 The Gitea Authors. All rights reserved.
// Copyright 2023 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// AuthorizationToken represents a authorization token to a user.
type AuthorizationToken struct {
	ID              int64  `xorm:"pk autoincr"`
	UID             int64  `xorm:"INDEX"`
	LookupKey       string `xorm:"INDEX UNIQUE"`
	HashedValidator string
	Expiry          timeutil.TimeStamp
}

// TableName provides the real table name.
func (AuthorizationToken) TableName() string {
	return "forgejo_auth_token"
}

func init() {
	db.RegisterModel(new(AuthorizationToken))
}

// IsExpired returns if the authorization token is expired.
func (authToken *AuthorizationToken) IsExpired() bool {
	return authToken.Expiry.AsLocalTime().Before(time.Now())
}

// GenerateAuthToken generates a new authentication token for the given user.
// It returns the lookup key and validator values that should be passed to the
// user via a long-term cookie.
func GenerateAuthToken(ctx context.Context, userID int64, expiry timeutil.TimeStamp) (lookupKey, validator string, err error) {
	// Request 64 random bytes. The first 32 bytes will be used for the lookupKey
	// and the other 32 bytes will be used for the validator.
	rBytes, err := util.CryptoRandomBytes(64)
	if err != nil {
		return "", "", err
	}
	hexEncoded := hex.EncodeToString(rBytes)
	validator, lookupKey = hexEncoded[64:], hexEncoded[:64]

	_, err = db.GetEngine(ctx).Insert(&AuthorizationToken{
		UID:             userID,
		Expiry:          expiry,
		LookupKey:       lookupKey,
		HashedValidator: HashValidator(rBytes[32:]),
	})
	return lookupKey, validator, err
}

// FindAuthToken will find a authorization token via the lookup key.
func FindAuthToken(ctx context.Context, lookupKey string) (*AuthorizationToken, error) {
	var authToken AuthorizationToken
	has, err := db.GetEngine(ctx).Where("lookup_key = ?", lookupKey).Get(&authToken)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("lookup key %q: %w", lookupKey, util.ErrNotExist)
	}
	return &authToken, nil
}

// DeleteAuthToken will delete the authorization token.
func DeleteAuthToken(ctx context.Context, authToken *AuthorizationToken) error {
	_, err := db.DeleteByBean(ctx, authToken)
	return err
}

// DeleteAuthTokenByUser will delete all authorization tokens for the user.
func DeleteAuthTokenByUser(ctx context.Context, userID int64) error {
	if userID == 0 {
		return nil
	}

	_, err := db.DeleteByBean(ctx, &AuthorizationToken{UID: userID})
	return err
}

// HashValidator will return a hexified hashed version of the validator.
func HashValidator(validator []byte) string {
	h := sha256.New()
	h.Write(validator)
	return hex.EncodeToString(h.Sum(nil))
}

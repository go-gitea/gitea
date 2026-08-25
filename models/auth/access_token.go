// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"encoding/hex"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// AccessToken represents a personal access token.
type AccessToken struct {
	ID             int64 `xorm:"pk autoincr"`
	UID            int64 `xorm:"INDEX"`
	Name           string
	Token          string `xorm:"-"`
	TokenHash      string `xorm:"UNIQUE"` // sha256 of token
	TokenSalt      string
	TokenLastEight string `xorm:"INDEX token_last_eight"`
	Scope          AccessTokenScope

	CreatedUnix       timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"INDEX updated"`
	HasRecentActivity bool               `xorm:"-"`
	HasUsed           bool               `xorm:"-"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (t *AccessToken) AfterLoad() {
	t.HasUsed = t.UpdatedUnix > t.CreatedUnix
	t.HasRecentActivity = t.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

func init() {
	db.RegisterModel(new(AccessToken))
}

// setNewTokenValue generates a fresh random token value and fills in its salt, hash, and last-eight.
func (t *AccessToken) setNewTokenValue() {
	salt := util.CryptoRandomString(10)
	token := util.CryptoRandomBytes(20)
	t.TokenSalt = salt
	t.Token = hex.EncodeToString(token)
	t.TokenHash = HashToken(t.Token, t.TokenSalt)
	t.TokenLastEight = t.Token[len(t.Token)-8:]
}

// NewAccessToken creates new access token.
func NewAccessToken(ctx context.Context, t *AccessToken) error {
	t.setNewTokenValue()
	_, err := db.GetEngine(ctx).Insert(t)
	return err
}

// RegenerateAccessToken regenerates the token value of an existing access token owned by userID, keeping its name and scope.
func RegenerateAccessToken(ctx context.Context, id, userID int64) (*AccessToken, error) {
	t := &AccessToken{}
	has, err := db.GetEngine(ctx).Where("id=? AND uid=?", id, userID).Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.NewNotExistErrorf("access token not found")
	}

	t.setNewTokenValue()
	if _, err := db.GetEngine(ctx).ID(t.ID).Cols("token_hash", "token_salt", "token_last_eight").NoAutoTime().Update(t); err != nil {
		return nil, err
	}
	return t, nil
}

// DisplayPublicOnly whether to display this as a public-only token.
func (t *AccessToken) DisplayPublicOnly() bool {
	publicOnly, err := t.Scope.PublicOnly()
	if err != nil {
		return false
	}
	return publicOnly
}

// GetAccessTokenBySHA returns access token by given token value
func GetAccessTokenBySHA(ctx context.Context, token string) (*AccessToken, error) {
	if len(token) < 8 {
		return nil, util.NewNotExistErrorf("access token not found")
	}

	cacheKey := "access:" + token
	lastEight := token[len(token)-8:]
	if cached, _ := TokenCache().Get(cacheKey); cached != nil {
		// Re-get the token from the db in case it has been deleted or regenerated in the intervening period
		accessToken := &AccessToken{}
		has, err := db.GetEngine(ctx).ID(cached.TokenID).Get(accessToken)
		if err != nil {
			return nil, err
		}
		if has && util.CryptoConstTimeEqual(accessToken.TokenHash, cached.TokenHash) {
			return accessToken, nil
		}
		// either the token has been deleted or changed, invalidate the cache
		TokenCache().Remove(cacheKey)
	}

	var tokens []AccessToken
	err := db.GetEngine(ctx).Table(&AccessToken{}).Where("token_last_eight = ?", lastEight).Find(&tokens)
	if err != nil {
		return nil, err
	} else if len(tokens) == 0 {
		return nil, util.NewNotExistErrorf("access token not found")
	}

	for _, t := range tokens {
		tempHash := HashToken(token, t.TokenSalt)
		if util.CryptoConstTimeEqual(t.TokenHash, tempHash) {
			TokenCache().Add(cacheKey, &TokenCacheItem{TokenID: t.ID, TokenHash: t.TokenHash})
			return &t, nil
		}
	}
	return nil, util.NewNotExistErrorf("access token not found")
}

// AccessTokenByNameExists checks if a token name has been used already by a user.
func AccessTokenByNameExists(ctx context.Context, token *AccessToken) (bool, error) {
	return db.GetEngine(ctx).Table("access_token").Where("name = ?", token.Name).And("uid = ?", token.UID).Exist()
}

// ListAccessTokensOptions contain filter options
type ListAccessTokensOptions struct {
	db.ListOptions
	Name   string
	UserID int64
}

func (opts ListAccessTokensOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	// user id is required, otherwise it will return all result which maybe a possible bug
	cond = cond.And(builder.Eq{"uid": opts.UserID})
	if len(opts.Name) > 0 {
		cond = cond.And(builder.Eq{"name": opts.Name})
	}
	return cond
}

func (opts ListAccessTokensOptions) ToOrders() string {
	return "created_unix DESC"
}

// UpdateAccessToken updates information of access token.
func UpdateAccessToken(ctx context.Context, t *AccessToken) error {
	_, err := db.GetEngine(ctx).ID(t.ID).AllCols().Update(t)
	return err
}

// DeleteAccessTokenByID deletes access token by given ID.
func DeleteAccessTokenByID(ctx context.Context, id, userID int64) error {
	cnt, err := db.GetEngine(ctx).ID(id).Delete(&AccessToken{UID: userID})
	if err != nil {
		return err
	} else if cnt != 1 {
		return util.NewNotExistErrorf("access token not found")
	}
	return nil
}

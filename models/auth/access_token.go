// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"encoding/hex"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/base"
	"gitea.dev/modules/log"
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
	TokenSalt      string // only set for legacy pbkdf2 hashes
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

// setNewTokenValue generates a fresh random token value and fills in its hash and last-eight.
func (t *AccessToken) setNewTokenValue() {
	t.Token = hex.EncodeToString(util.CryptoRandomBytes(20))
	t.TokenHash, t.TokenSalt = base.EncodeSha256(t.Token), ""
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

	var tokens []AccessToken
	if err := db.GetEngine(ctx).Where("token_last_eight = ?", token[len(token)-8:]).Find(&tokens); err != nil {
		return nil, err
	}
	for _, t := range tokens {
		if VerifyToken(ctx, &t, token, t.TokenSalt, t.TokenHash) {
			return &t, nil
		}
	}
	return nil, util.NewNotExistErrorf("access token not found")
}

// VerifyToken checks a token against its row's hash and replaces a legacy pbkdf2 hash with sha256 unless the token was regenerated meanwhile.
func VerifyToken(ctx context.Context, bean any, token, salt, hash string) bool {
	tokenHash := base.EncodeSha256(token)
	if len(hash) == len(tokenHash) {
		return util.CryptoConstTimeEqual(hash, tokenHash)
	}
	if !util.CryptoConstTimeEqual(hash, HashToken(token, salt)) {
		return false
	}
	if _, err := db.GetEngine(ctx).Table(bean).Where("token_hash = ?", hash).NoAutoTime().
		Update(map[string]any{"token_hash": tokenHash, "token_salt": ""}); err != nil {
		log.Error("VerifyToken: %v", err)
	}
	return true
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

// TouchAccessToken records a token use at most once a minute, re-checked in SQL so concurrent stale copies write once.
func TouchAccessToken(ctx context.Context, t *AccessToken) error {
	const intervalSeconds = 60
	now := timeutil.TimeStampNow()
	if t.UpdatedUnix > t.CreatedUnix && now-t.UpdatedUnix < intervalSeconds {
		return nil
	}
	_, err := db.GetEngine(ctx).Where("id = ? AND (updated_unix <= created_unix OR updated_unix <= ?)", t.ID, now-intervalSeconds).
		Cols("updated_unix").NoAutoTime().Update(&AccessToken{UpdatedUnix: max(now, t.CreatedUnix+1)})
	return err
}

// GetAccessTokenByID returns the access token with the given ID owned by userID.
func GetAccessTokenByID(ctx context.Context, id, userID int64) (*AccessToken, error) {
	t, has, err := db.Get[AccessToken](ctx, builder.Eq{"id": id, "uid": userID})
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.NewNotExistErrorf("access token not found")
	}
	return t, nil
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

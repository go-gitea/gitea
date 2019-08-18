// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/subtle"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/timeutil"

	gouuid "github.com/satori/go.uuid"
)

// AccessToken represents a personal access token.
type AccessToken struct {
	ID             int64 `xorm:"pk autoincr"`
	UID            int64 `xorm:"INDEX"`
	Name           string
	Token          string `xorm:"-"`
	TokenHash      string `xorm:"UNIQUE"` // sha256 of token
	TokenSalt      string
	TokenLastEight string `xorm:"token_last_eight"`

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

// NewAccessToken creates new access token.
func NewAccessToken(t *AccessToken) error {
	salt, err := generate.GetRandomString(10)
	if err != nil {
		return err
	}
	t.TokenSalt = salt
	t.Token = base.EncodeSha1(gouuid.NewV4().String())
	t.TokenHash = hashToken(t.Token, t.TokenSalt)
	t.TokenLastEight = t.Token[len(t.Token)-8:]
	_, err = x.Insert(t)
	return err
}

// GetAccessTokenBySHA returns access token by given token value
func GetAccessTokenBySHA(token string) (*AccessToken, error) {
	if token == "" {
		return nil, ErrAccessTokenEmpty{}
	}
	if len(token) < 8 {
		return nil, ErrAccessTokenNotExist{token}
	}
	var tokens []AccessToken
	lastEight := token[len(token)-8:]
	err := x.Table(&AccessToken{}).Where("token_last_eight = ?", lastEight).Find(&tokens)
	if err != nil {
		return nil, err
	} else if len(tokens) == 0 {
		return nil, ErrAccessTokenNotExist{token}
	}
	for _, t := range tokens {
		tempHash := hashToken(token, t.TokenSalt)
		if subtle.ConstantTimeCompare([]byte(t.TokenHash), []byte(tempHash)) == 1 {
			return &t, nil
		}
	}
	return nil, ErrAccessTokenNotExist{token}
}

// ListAccessTokens returns a list of access tokens belongs to given user.
func ListAccessTokens(uid int64) ([]*AccessToken, error) {
	tokens := make([]*AccessToken, 0, 5)
	return tokens, x.
		Where("uid=?", uid).
		Desc("id").
		Find(&tokens)
}

// UpdateAccessToken updates information of access token.
func UpdateAccessToken(t *AccessToken) error {
	_, err := x.ID(t.ID).AllCols().Update(t)
	return err
}

// DeleteAccessTokenByID deletes access token by given ID.
func DeleteAccessTokenByID(id, userID int64) error {
	cnt, err := x.ID(id).Delete(&AccessToken{
		UID: userID,
	})
	if err != nil {
		return err
	} else if cnt != 1 {
		return ErrAccessTokenNotExist{}
	}
	return nil
}

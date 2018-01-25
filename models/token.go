// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	gouuid "github.com/satori/go.uuid"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/util"
)

// AccessToken represents a personal access token.
type AccessToken struct {
	ID   int64 `xorm:"pk autoincr"`
	UID  int64 `xorm:"INDEX"`
	Name string
	Sha1 string `xorm:"UNIQUE VARCHAR(40)"`

	CreatedUnix       util.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix       util.TimeStamp `xorm:"INDEX updated"`
	HasRecentActivity bool           `xorm:"-"`
	HasUsed           bool           `xorm:"-"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (t *AccessToken) AfterLoad() {
	t.HasUsed = t.UpdatedUnix > t.CreatedUnix
	t.HasRecentActivity = t.UpdatedUnix.AddDuration(7*24*time.Hour) > util.TimeStampNow()
}

// NewAccessToken creates new access token.
func NewAccessToken(t *AccessToken) error {
	t.Sha1 = base.EncodeSha1(gouuid.NewV4().String())
	_, err := x.Insert(t)
	return err
}

// GetAccessTokenBySHA returns access token by given sha1.
func GetAccessTokenBySHA(sha string) (*AccessToken, error) {
	if sha == "" {
		return nil, ErrAccessTokenEmpty{}
	}
	t := &AccessToken{Sha1: sha}
	has, err := x.Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrAccessTokenNotExist{sha}
	}
	return t, nil
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

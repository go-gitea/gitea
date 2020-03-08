// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/setting"
)

// EmailHash represents a pre-generated hash map
type EmailHash struct {
	Hash  string `xorm:"pk varchar(32)"`
	Email string `xorm:"UNIQUE NOT NULL"`
}

// GetEmailForHash converts a provided md5sum to the email
func GetEmailForHash(md5Sum string) (string, error) {
	return cache.GetString("Avatar:"+md5Sum, func() (string, error) {
		emailHash := EmailHash{
			Hash: strings.ToLower(strings.TrimSpace(md5Sum)),
		}

		_, err := x.Get(&emailHash)
		return emailHash.Email, err
	})
}

// AvatarLink returns an avatar link for a provided email
func AvatarLink(email string) string {
	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	sum := fmt.Sprintf("%x", md5.Sum([]byte(lowerEmail)))
	_, _ = cache.GetString("Avatar:"+sum, func() (string, error) {
		emailHash := &EmailHash{
			Email: lowerEmail,
			Hash:  sum,
		}
		_, _ = x.Insert(emailHash)
		return lowerEmail, nil
	})
	return setting.AppSubURL + "/avatar/" + url.PathEscape(sum)
}

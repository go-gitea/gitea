// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// EmailHash represents a pre-generated hash map
type EmailHash struct {
	Hash  string `xorm:"pk varchar(32)"`
	Email string `xorm:"UNIQUE NOT NULL"`
}

// DefaultAvatarLink the default avatar link
func DefaultAvatarLink() string {
	return setting.AppSubURL + "/img/avatar_default.png"
}

// DefaultAvatarSize is a sentinel value for the default avatar size, as
// determined by the avatar-hosting service.
const DefaultAvatarSize = -1

// HashEmail hashes email address to MD5 string.
// https://en.gravatar.com/site/implement/hash/
func HashEmail(email string) string {
	return base.EncodeMD5(strings.ToLower(strings.TrimSpace(email)))
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

// LibravatarURL returns the URL for the given email. This function should only
// be called if a federated avatar service is enabled.
func LibravatarURL(email string) (*url.URL, error) {
	urlStr, err := setting.LibravatarService.FromEmail(email)
	if err != nil {
		log.Error("LibravatarService.FromEmail(email=%s): error %v", email, err)
		return nil, err
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Error("Failed to parse libravatar url(%s): error %v", urlStr, err)
		return nil, err
	}
	return u, nil
}

// HashedAvatarLink returns an avatar link for a provided email
func HashedAvatarLink(email string) string {
	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	sum := fmt.Sprintf("%x", md5.Sum([]byte(lowerEmail)))
	_, _ = cache.GetString("Avatar:"+sum, func() (string, error) {
		emailHash := &EmailHash{
			Email: lowerEmail,
			Hash:  sum,
		}
		// OK we're going to open a session just because I think that that might hide away any problems with postgres reporting errors
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			// we don't care about any DB problem just return the lowerEmail
			return lowerEmail, nil
		}
		_, _ = sess.Insert(emailHash)
		if err := sess.Commit(); err != nil {
			// Seriously we don't care about any DB problems just return the lowerEmail - we expect the transaction to fail most of the time
			return lowerEmail, nil
		}
		return lowerEmail, nil
	})
	return setting.AppSubURL + "/avatar/" + url.PathEscape(sum)
}

// MakeFinalAvatarUrl constructs the final URL string from a net.URL
func MakeFinalAvatarUrl(u *url.URL, size int) string {
	vals := u.Query()
	vals.Set("d", "identicon")
	if size != DefaultAvatarSize {
		vals.Set("s", strconv.Itoa(size))
	}
	u.RawQuery = vals.Encode()
	return u.String()
}

// SizedAvatarLink returns a sized link to the avatar for the given email address.
func SizedAvatarLink(email string, size int) string {
	var avatarURL *url.URL
	if setting.EnableFederatedAvatar && setting.LibravatarService != nil {
		// this is the slow path that would need to call libravatarURL() which
		// does DNS lookups. avoid it by issueing a redirect.
		return HashedAvatarLink(email)
	} else if !setting.DisableGravatar {
		// copy GravatarSourceURL, because we will modify its Path.
		copyOfGravatarSourceURL := *setting.GravatarSourceURL
		avatarURL = &copyOfGravatarSourceURL
		avatarURL.Path = path.Join(avatarURL.Path, HashEmail(email))
	} else {
		return DefaultAvatarLink()
	}

	return MakeFinalAvatarUrl(avatarURL, size)
}

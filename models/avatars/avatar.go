// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package avatars

import (
	"net/url"
	"path"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// DefaultAvatarSize is a sentinel value for the default avatar size, as determined by the avatar-hosting service.
// in history the value was "-1", it's not handy as "0", because the int value of empty param is always "0"
const DefaultAvatarSize = 0

// DefaultAvatarPixelSize is the default size in pixels of a rendered avatar
const DefaultAvatarPixelSize = 28

// AvatarRenderedSizeFactor is the factor by which the default size is increased for finer rendering
const AvatarRenderedSizeFactor = 4

// EmailHash represents a pre-generated hash map (mainly used by LibravatarURL, it queries email server's DNS records)
type EmailHash struct {
	Hash  string `xorm:"pk varchar(32)"`
	Email string `xorm:"UNIQUE NOT NULL"`
}

func init() {
	db.RegisterModel(new(EmailHash))
}

// DefaultAvatarLink the default avatar link
func DefaultAvatarLink() string {
	u, err := url.Parse(setting.AppSubURL)
	if err != nil {
		log.Error("GetUserByEmail: %v", err)
		return ""
	}

	u.Path = path.Join(u.Path, "/assets/img/avatar_default.png")
	return u.String()
}

// HashEmail hashes email address to MD5 string. https://en.gravatar.com/site/implement/hash/
func HashEmail(email string) string {
	return base.EncodeMD5(strings.ToLower(strings.TrimSpace(email)))
}

// GetEmailForHash converts a provided md5sum to the email
func GetEmailForHash(md5Sum string) (string, error) {
	return cache.GetString("Avatar:"+md5Sum, func() (string, error) {
		emailHash := EmailHash{
			Hash: strings.ToLower(strings.TrimSpace(md5Sum)),
		}

		_, err := db.DefaultContext().Engine().Get(&emailHash)
		return emailHash.Email, err
	})
}

// LibravatarURL returns the URL for the given email. Slow due to the DNS lookup.
// This function should only be called if a federated avatar service is enabled.
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

// saveEmailHash returns an avatar link for a provided email,
// the email and hash are saved into database, which will be used by GetEmailForHash later
func saveEmailHash(email string) string {
	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	emailHash := HashEmail(lowerEmail)
	_, _ = cache.GetString("Avatar:"+emailHash, func() (string, error) {
		emailHash := &EmailHash{
			Email: lowerEmail,
			Hash:  emailHash,
		}
		// OK we're going to open a session just because I think that that might hide away any problems with postgres reporting errors
		if err := db.WithTx(func(ctx *db.Context) error {
			has, err := ctx.Engine().Where("email = ? AND hash = ?", emailHash.Email, emailHash.Hash).Get(new(EmailHash))
			if has || err != nil {
				// Seriously we don't care about any DB problems just return the lowerEmail - we expect the transaction to fail most of the time
				return nil
			}
			_, _ = ctx.Engine().Insert(emailHash)
			return nil
		}); err != nil {
			// Seriously we don't care about any DB problems just return the lowerEmail - we expect the transaction to fail most of the time
			return lowerEmail, nil
		}
		return lowerEmail, nil
	})
	return emailHash
}

// GenerateUserAvatarFastLink returns a fast link to the user's avatar via the local explore page.
func GenerateUserAvatarFastLink(userName string, size int) string {
	if size < 0 {
		size = 0
	}
	link := setting.AppSubURL + "/user/avatar/" + userName + "/" + strconv.Itoa(size)
	return setting.AppURL + strings.TrimPrefix(link, setting.AppSubURL)[1:]
}

// generateEmailAvatarLink returns a email avatar link.
// if final is true, it may use a slow path (eg: query DNS).
// if final is false, it always uses a fast path.
func generateEmailAvatarLink(email string, size int, final bool) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return DefaultAvatarLink()
	}

	var avatarURL *url.URL
	var err error

	if setting.EnableFederatedAvatar && setting.LibravatarService != nil {
		emailHash := saveEmailHash(email)
		if final {
			if avatarURL, err = LibravatarURL(email); err != nil {
				return DefaultAvatarLink()
			}
		} else {
			if size > 0 {
				return setting.AppSubURL + "/avatar/" + emailHash + "?size=" + strconv.Itoa(size)
			}
			return setting.AppSubURL + "/avatar/" + emailHash
		}
	} else if !setting.DisableGravatar {
		avatarURLDummy := *setting.GravatarSourceURL // copy GravatarSourceURL, because we will modify its Path.
		avatarURL = &avatarURLDummy
		avatarURL.Path = path.Join(avatarURL.Path, HashEmail(email))
	} else {
		return DefaultAvatarLink()
	}

	urlQuery := avatarURL.Query()
	urlQuery.Set("d", "identicon")
	if size > 0 {
		urlQuery.Set("s", strconv.Itoa(size))
	}
	avatarURL.RawQuery = urlQuery.Encode()
	return avatarURL.String()
}

//GenerateEmailAvatarFastLink returns a avatar link (fast, the link may be a delegated one)
func GenerateEmailAvatarFastLink(email string, size int) string {
	return generateEmailAvatarLink(email, size, false)
}

//GenerateEmailAvatarFinalLink returns a avatar final link (maybe slow)
func GenerateEmailAvatarFinalLink(email string, size int) string {
	return generateEmailAvatarLink(email, size, true)
}

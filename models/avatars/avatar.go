// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatars

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"strk.kbt.io/projects/go/libravatar"
)

const (
	// DefaultAvatarClass is the default class of a rendered avatar
	DefaultAvatarClass = "ui avatar gt-vm"
	// DefaultAvatarPixelSize is the default size in pixels of a rendered avatar
	DefaultAvatarPixelSize = 28
)

// EmailHash represents a pre-generated hash map (mainly used by LibravatarURL, it queries email server's DNS records)
type EmailHash struct {
	Hash  string `xorm:"pk varchar(32)"`
	Email string `xorm:"UNIQUE NOT NULL"`
}

func init() {
	db.RegisterModel(new(EmailHash))
}

type avatarSettingStruct struct {
	defaultAvatarLink string
	gravatarSource    string
	gravatarSourceURL *url.URL
	libravatar        *libravatar.Libravatar
}

var avatarSettingAtomic atomic.Pointer[avatarSettingStruct]

func loadAvatarSetting() (*avatarSettingStruct, error) {
	s := avatarSettingAtomic.Load()
	if s == nil || s.gravatarSource != setting.GravatarSource {
		s = &avatarSettingStruct{}
		u, err := url.Parse(setting.AppSubURL)
		if err != nil {
			return nil, fmt.Errorf("unable to parse AppSubURL: %w", err)
		}

		u.Path = path.Join(u.Path, "/assets/img/avatar_default.png")
		s.defaultAvatarLink = u.String()

		s.gravatarSourceURL, err = url.Parse(setting.GravatarSource)
		if err != nil {
			return nil, fmt.Errorf("unable to parse GravatarSource %q: %w", setting.GravatarSource, err)
		}

		s.libravatar = libravatar.New()
		if s.gravatarSourceURL.Scheme == "https" {
			s.libravatar.SetUseHTTPS(true)
			s.libravatar.SetSecureFallbackHost(s.gravatarSourceURL.Host)
		} else {
			s.libravatar.SetUseHTTPS(false)
			s.libravatar.SetFallbackHost(s.gravatarSourceURL.Host)
		}

		avatarSettingAtomic.Store(s)
	}
	return s, nil
}

// DefaultAvatarLink the default avatar link
func DefaultAvatarLink() string {
	a, err := loadAvatarSetting()
	if err != nil {
		log.Error("Failed to loadAvatarSetting: %v", err)
		return ""
	}
	return a.defaultAvatarLink
}

// HashEmail hashes email address to MD5 string. https://en.gravatar.com/site/implement/hash/
func HashEmail(email string) string {
	return base.EncodeMD5(strings.ToLower(strings.TrimSpace(email)))
}

// GetEmailForHash converts a provided md5sum to the email
func GetEmailForHash(ctx context.Context, md5Sum string) (string, error) {
	return cache.GetString("Avatar:"+md5Sum, func() (string, error) {
		emailHash := EmailHash{
			Hash: strings.ToLower(strings.TrimSpace(md5Sum)),
		}

		_, err := db.GetEngine(ctx).Get(&emailHash)
		return emailHash.Email, err
	})
}

// LibravatarURL returns the URL for the given email. Slow due to the DNS lookup.
// This function should only be called if a federated avatar service is enabled.
func LibravatarURL(email string) (*url.URL, error) {
	a, err := loadAvatarSetting()
	if err != nil {
		return nil, err
	}
	urlStr, err := a.libravatar.FromEmail(email)
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
func saveEmailHash(ctx context.Context, email string) string {
	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	emailHash := HashEmail(lowerEmail)
	_, _ = cache.GetString("Avatar:"+emailHash, func() (string, error) {
		emailHash := &EmailHash{
			Email: lowerEmail,
			Hash:  emailHash,
		}
		// OK we're going to open a session just because I think that that might hide away any problems with postgres reporting errors
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			has, err := db.GetEngine(ctx).Where("email = ? AND hash = ?", emailHash.Email, emailHash.Hash).Get(new(EmailHash))
			if has || err != nil {
				// Seriously we don't care about any DB problems just return the lowerEmail - we expect the transaction to fail most of the time
				return nil
			}
			_, _ = db.GetEngine(ctx).Insert(emailHash)
			return nil
		}); err != nil {
			// Seriously we don't care about any DB problems just return the lowerEmail - we expect the transaction to fail most of the time
			return lowerEmail, nil
		}
		return lowerEmail, nil
	})
	return emailHash
}

// GenerateUserAvatarFastLink returns a fast link (302) to the user's avatar: "/user/avatar/${User.Name}/${size}"
func GenerateUserAvatarFastLink(userName string, size int) string {
	if size < 0 {
		size = 0
	}
	return setting.AppSubURL + "/user/avatar/" + url.PathEscape(userName) + "/" + strconv.Itoa(size)
}

// GenerateUserAvatarImageLink returns a link for `User.Avatar` image file: "/avatars/${User.Avatar}"
func GenerateUserAvatarImageLink(userAvatar string, size int) string {
	if size > 0 {
		return setting.AppSubURL + "/avatars/" + url.PathEscape(userAvatar) + "?size=" + strconv.Itoa(size)
	}
	return setting.AppSubURL + "/avatars/" + url.PathEscape(userAvatar)
}

// generateRecognizedAvatarURL generate a recognized avatar (Gravatar/Libravatar) URL, it modifies the URL so the parameter is passed by a copy
func generateRecognizedAvatarURL(u url.URL, size int) string {
	urlQuery := u.Query()
	urlQuery.Set("d", "identicon")
	if size > 0 {
		urlQuery.Set("s", strconv.Itoa(size))
	}
	u.RawQuery = urlQuery.Encode()
	return u.String()
}

// generateEmailAvatarLink returns a email avatar link.
// if final is true, it may use a slow path (eg: query DNS).
// if final is false, it always uses a fast path.
func generateEmailAvatarLink(ctx context.Context, email string, size int, final bool) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return DefaultAvatarLink()
	}

	avatarSetting, err := loadAvatarSetting()
	if err != nil {
		return DefaultAvatarLink()
	}

	enableFederatedAvatar := setting.Config().Picture.EnableFederatedAvatar.Value(ctx)
	if enableFederatedAvatar {
		emailHash := saveEmailHash(ctx, email)
		if final {
			// for final link, we can spend more time on slow external query
			var avatarURL *url.URL
			if avatarURL, err = LibravatarURL(email); err != nil {
				return DefaultAvatarLink()
			}
			return generateRecognizedAvatarURL(*avatarURL, size)
		}
		// for non-final link, we should return fast (use a 302 redirection link)
		urlStr := setting.AppSubURL + "/avatar/" + url.PathEscape(emailHash)
		if size > 0 {
			urlStr += "?size=" + strconv.Itoa(size)
		}
		return urlStr
	}

	disableGravatar := setting.Config().Picture.DisableGravatar.Value(ctx)
	if !disableGravatar {
		// copy GravatarSourceURL, because we will modify its Path.
		avatarURLCopy := *avatarSetting.gravatarSourceURL
		avatarURLCopy.Path = path.Join(avatarURLCopy.Path, HashEmail(email))
		return generateRecognizedAvatarURL(avatarURLCopy, size)
	}

	return DefaultAvatarLink()
}

// GenerateEmailAvatarFastLink returns a avatar link (fast, the link may be a delegated one: "/avatar/${hash}")
func GenerateEmailAvatarFastLink(ctx context.Context, email string, size int) string {
	return generateEmailAvatarLink(ctx, email, size, false)
}

// GenerateEmailAvatarFinalLink returns a avatar final link (maybe slow)
func GenerateEmailAvatarFinalLink(ctx context.Context, email string, size int) string {
	return generateEmailAvatarLink(ctx, email, size, true)
}

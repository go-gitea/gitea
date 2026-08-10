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

	"gitea.dev/models/db"
	"gitea.dev/modules/avatar"
	"gitea.dev/modules/base"
	"gitea.dev/modules/cache"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"xorm.io/builder"
)

const (
	// DefaultAvatarClass is the default class of a rendered avatar
	DefaultAvatarClass = "ui avatar tw-align-middle"
	// DefaultAvatarPixelSize is the default size in pixels of a rendered avatar
	DefaultAvatarPixelSize = 28
)

const emailHashType = "sha256" // so a later algorithm change can tell old rows apart

// EmailHash keeps the email out of the rendered page
type EmailHash struct {
	Hash     string `xorm:"pk varchar(64)"`
	HashType string `xorm:"UNIQUE(email_type) NOT NULL varchar(16)"`
	Email    string `xorm:"UNIQUE(email_type) NOT NULL"`
}

func init() {
	db.RegisterModel(new(EmailHash))
}

type avatarSettingStruct struct {
	appSubURL         string
	gravatarSource    string
	defaultAvatarLink string
	gravatarSourceURL *url.URL
}

var avatarSettingAtomic atomic.Pointer[avatarSettingStruct]

func loadAvatarSetting() (*avatarSettingStruct, error) {
	s := avatarSettingAtomic.Load()
	if s != nil && s.appSubURL == setting.AppSubURL && s.gravatarSource == setting.GravatarSource {
		return s, nil
	}

	u, err := url.Parse(setting.AppSubURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse AppSubURL: %w", err)
	}
	u.Path = path.Join(u.Path, "/assets/img/avatar_default.png")

	gravatarSourceURL, err := url.Parse(setting.GravatarSource)
	if err != nil {
		return nil, fmt.Errorf("unable to parse GravatarSource %q: %w", setting.GravatarSource, err)
	}

	s = &avatarSettingStruct{
		appSubURL:         setting.AppSubURL,
		gravatarSource:    setting.GravatarSource,
		defaultAvatarLink: u.String(),
		gravatarSourceURL: gravatarSourceURL,
	}
	avatarSettingAtomic.Store(s)
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

// HashEmail hashes an email address the way avatar services address it. https://docs.gravatar.com/api/avatars/images/
func HashEmail(email string) string {
	return base.EncodeSha256(strings.ToLower(strings.TrimSpace(email)))
}

// GetEmailForHash converts a provided hash to the email
func GetEmailForHash(ctx context.Context, hash string) (string, error) {
	return cache.GetString("Avatar:"+hash, func() (string, error) {
		emailHash, has, err := db.Get[EmailHash](ctx, builder.Eq{"`hash`": strings.ToLower(strings.TrimSpace(hash))})
		if err != nil {
			return "", err
		} else if !has {
			return "", nil
		}
		return emailHash.Email, nil
	})
}

// saveEmailHash returns the hash and stores the pair for GetEmailForHash
func saveEmailHash(ctx context.Context, email string) string {
	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	emailHash := HashEmail(lowerEmail)
	// the cache entry doubles as the "already stored" marker
	_, _ = cache.GetString("Avatar:"+emailHash, func() (string, error) {
		// a session keeps a duplicate key error away from an outer transaction
		_ = db.WithTx(ctx, func(ctx context.Context) error {
			has, err := db.Exist[EmailHash](ctx, builder.Eq{"email": lowerEmail, "`hash`": emailHash})
			if has || err != nil {
				return nil
			}
			_, _ = db.GetEngine(ctx).Insert(&EmailHash{Email: lowerEmail, Hash: emailHash, HashType: emailHashType})
			return nil
		})
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

func generateSourceAvatarURL(source url.URL, email string, size int) string {
	source.Path = path.Join(source.Path, HashEmail(email))
	urlQuery := source.Query()
	urlQuery.Set("d", "identicon")
	if size > 0 {
		urlQuery.Set("s", strconv.Itoa(size))
	}
	source.RawQuery = urlQuery.Encode()
	return source.String()
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

	if setting.Config().Picture.EnableFederatedAvatar.Value(ctx) {
		if !final {
			// return a 302 link, so page rendering never waits for the DNS query
			link := setting.AppSubURL + "/avatar/" + url.PathEscape(saveEmailHash(ctx, email))
			if size > 0 {
				link += "?size=" + strconv.Itoa(size)
			}
			return link
		}
		source := *avatarSetting.gravatarSourceURL
		if host := avatar.LookupFederatedHost(ctx, email, source.Scheme == "https"); host != "" {
			source.Host, source.Path = host, "/avatar"
		}
		return generateSourceAvatarURL(source, email, size)
	}

	if !setting.Config().Picture.DisableGravatar.Value(ctx) {
		return generateSourceAvatarURL(*avatarSetting.gravatarSourceURL, email, size)
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

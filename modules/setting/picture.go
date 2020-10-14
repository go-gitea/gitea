// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net/url"

	"code.gitea.io/gitea/modules/log"

	"strk.kbt.io/projects/go/libravatar"
)

// settings
var (
	// Picture settings
	Avatar = struct {
		Storage

		MaxWidth    int
		MaxHeight   int
		MaxFileSize int64
	}{
		MaxWidth:    4096,
		MaxHeight:   3072,
		MaxFileSize: 1048576,
	}

	GravatarSource        string
	GravatarSourceURL     *url.URL
	DisableGravatar       bool
	EnableFederatedAvatar bool
	LibravatarService     *libravatar.Libravatar

	RepoAvatar = struct {
		Storage

		Fallback      string
		FallbackImage string
	}{}
)

func newPictureService() {
	sec := Cfg.Section("picture")

	avatarSec := Cfg.Section("avatar")
	storageType := sec.Key("AVATAR_STORAGE_TYPE").MustString("")
	// Specifically default PATH to AVATAR_UPLOAD_PATH
	avatarSec.Key("PATH").MustString(
		sec.Key("AVATAR_UPLOAD_PATH").String())

	Avatar.Storage = getStorage("avatars", storageType, avatarSec)

	Avatar.MaxWidth = sec.Key("AVATAR_MAX_WIDTH").MustInt(4096)
	Avatar.MaxHeight = sec.Key("AVATAR_MAX_HEIGHT").MustInt(3072)
	Avatar.MaxFileSize = sec.Key("AVATAR_MAX_FILE_SIZE").MustInt64(1048576)

	switch source := sec.Key("GRAVATAR_SOURCE").MustString("gravatar"); source {
	case "duoshuo":
		GravatarSource = "http://gravatar.duoshuo.com/avatar/"
	case "gravatar":
		GravatarSource = "https://secure.gravatar.com/avatar/"
	case "libravatar":
		GravatarSource = "https://seccdn.libravatar.org/avatar/"
	default:
		GravatarSource = source
	}
	DisableGravatar = sec.Key("DISABLE_GRAVATAR").MustBool()
	EnableFederatedAvatar = sec.Key("ENABLE_FEDERATED_AVATAR").MustBool(!InstallLock)
	if OfflineMode {
		DisableGravatar = true
		EnableFederatedAvatar = false
	}
	if DisableGravatar {
		EnableFederatedAvatar = false
	}
	if EnableFederatedAvatar || !DisableGravatar {
		var err error
		GravatarSourceURL, err = url.Parse(GravatarSource)
		if err != nil {
			log.Fatal("Failed to parse Gravatar URL(%s): %v",
				GravatarSource, err)
		}
	}

	if EnableFederatedAvatar {
		LibravatarService = libravatar.New()
		if GravatarSourceURL.Scheme == "https" {
			LibravatarService.SetUseHTTPS(true)
			LibravatarService.SetSecureFallbackHost(GravatarSourceURL.Host)
		} else {
			LibravatarService.SetUseHTTPS(false)
			LibravatarService.SetFallbackHost(GravatarSourceURL.Host)
		}
	}

	newRepoAvatarService()
}

func newRepoAvatarService() {
	sec := Cfg.Section("picture")

	repoAvatarSec := Cfg.Section("repo-avatar")
	storageType := sec.Key("REPOSITORY_AVATAR_STORAGE_TYPE").MustString("")
	// Specifically default PATH to AVATAR_UPLOAD_PATH
	repoAvatarSec.Key("PATH").MustString(
		sec.Key("REPOSITORY_AVATAR_UPLOAD_PATH").String())

	RepoAvatar.Storage = getStorage("repo-avatars", storageType, repoAvatarSec)

	RepoAvatar.Fallback = sec.Key("REPOSITORY_AVATAR_FALLBACK").MustString("none")
	RepoAvatar.FallbackImage = sec.Key("REPOSITORY_AVATAR_FALLBACK_IMAGE").MustString("/img/repo_default.png")
}

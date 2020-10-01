// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net/url"
	"path/filepath"

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
		Storage: Storage{
			Type:        LocalStorageType,
			ServeDirect: false,
		},
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
	}{
		Storage: Storage{
			Type:        LocalStorageType,
			ServeDirect: false,
		},
	}
)

func newPictureService() {
	sec := Cfg.Section("picture")

	Avatar.Storage.Type = sec.Key("AVATAR_STORAGE_TYPE").MustString("")
	if Avatar.Storage.Type == "" {
		Avatar.Storage.Type = "default"
	}

	storage, ok := storages[Avatar.Storage.Type]
	if !ok {
		log.Fatal("Failed to get avatar storage type: %s", Avatar.Storage.Type)
	}
	Avatar.Storage = storage

	switch Avatar.Storage.Type {
	case LocalStorageType:
		Avatar.Path = sec.Key("AVATAR_UPLOAD_PATH").MustString(Avatar.Path)
		if Avatar.Path == "" {
			Avatar.Path = filepath.Join(AppDataPath, "avatars")
		}
		forcePathSeparator(Avatar.Path)
		if !filepath.IsAbs(Avatar.Path) {
			Avatar.Path = filepath.Join(AppWorkPath, Avatar.Path)
		}
	case MinioStorageType:
		Avatar.Minio.BasePath = sec.Key("AVATAR_UPLOAD_PATH").MustString("avatars/")
	}

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

	RepoAvatar.Storage.Type = sec.Key("REPOSITORY_AVATAR_STORAGE_TYPE").MustString("")
	if RepoAvatar.Storage.Type == "" {
		RepoAvatar.Storage.Type = "default"
	}

	storage, ok := storages[RepoAvatar.Storage.Type]
	if !ok {
		log.Fatal("Failed to get repo-avatar storage type: %s", RepoAvatar.Storage.Type)
	}
	RepoAvatar.Storage = storage

	switch RepoAvatar.Storage.Type {
	case LocalStorageType:
		RepoAvatar.Path = sec.Key("REPOSITORY_AVATAR_UPLOAD_PATH").MustString(filepath.Join(AppDataPath, "repo-avatars"))
		forcePathSeparator(RepoAvatar.Path)
		if !filepath.IsAbs(RepoAvatar.Path) {
			RepoAvatar.Path = filepath.Join(AppWorkPath, RepoAvatar.Path)
		}
	case MinioStorageType:
		RepoAvatar.Minio.BasePath = sec.Key("REPOSITORY_AVATAR_MINIO_BASE_PATH").MustString("repo-avatars/")
	}

	RepoAvatar.Fallback = sec.Key("REPOSITORY_AVATAR_FALLBACK").MustString("none")
	RepoAvatar.FallbackImage = sec.Key("REPOSITORY_AVATAR_FALLBACK_IMAGE").MustString("/img/repo_default.png")
}

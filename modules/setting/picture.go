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
	AvatarMaxFileSize     int64

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

	Avatar.Storage.Type = sec.Key("AVATAR_STORE_TYPE").MustString("")
	if Avatar.Storage.Type == "" {
		Avatar.Storage.Type = "default"
	}

	if Avatar.Storage.Type != LocalStorageType && Avatar.Storage.Type != MinioStorageType {
		storage, ok := storages[Avatar.Storage.Type]
		if !ok {
			log.Fatal("Failed to get avatar storage type: %s", Avatar.Storage.Type)
		}
		Avatar.Storage = storage
	}

	// Override
	Avatar.ServeDirect = sec.Key("AVATAR_SERVE_DIRECT").MustBool(Attachment.ServeDirect)

	switch Avatar.Storage.Type {
	case LocalStorageType:
		Avatar.Path = sec.Key("AVATAR_UPLOAD_PATH").MustString(filepath.Join(AppDataPath, "avatars"))
		forcePathSeparator(Avatar.Path)
		if !filepath.IsAbs(Avatar.Path) {
			Avatar.Path = filepath.Join(AppWorkPath, Avatar.Path)
		}
	case MinioStorageType:
		Avatar.Minio.Endpoint = sec.Key("AVATAR_MINIO_ENDPOINT").MustString(Avatar.Minio.Endpoint)
		Avatar.Minio.AccessKeyID = sec.Key("AVATAR_MINIO_ACCESS_KEY_ID").MustString(Avatar.Minio.AccessKeyID)
		Avatar.Minio.SecretAccessKey = sec.Key("AVATAR_MINIO_SECRET_ACCESS_KEY").MustString(Avatar.Minio.SecretAccessKey)
		Avatar.Minio.Bucket = sec.Key("AVATAR_MINIO_BUCKET").MustString(Avatar.Minio.Bucket)
		Avatar.Minio.Location = sec.Key("AVATAR_MINIO_LOCATION").MustString(Avatar.Minio.Location)
		Avatar.Minio.UseSSL = sec.Key("AVATAR_MINIO_USE_SSL").MustBool(Avatar.Minio.UseSSL)
		Avatar.Minio.BasePath = sec.Key("AVATAR_MINIO_BASE_PATH").MustString("avatars/")
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

	RepoAvatar.Storage.Type = sec.Key("REPOSITORY_AVATAR_STORE_TYPE").MustString("")
	if RepoAvatar.Storage.Type == "" {
		RepoAvatar.Storage.Type = "default"
	}

	if RepoAvatar.Storage.Type != LocalStorageType && RepoAvatar.Storage.Type != MinioStorageType {
		storage, ok := storages[RepoAvatar.Storage.Type]
		if !ok {
			log.Fatal("Failed to get repo-avatar storage type: %s", RepoAvatar.Storage.Type)
		}
		RepoAvatar.Storage = storage
	}

	// Override
	RepoAvatar.ServeDirect = sec.Key("REPOSITORY_AVATAR_SERVE_DIRECT").MustBool(Attachment.ServeDirect)

	switch RepoAvatar.Storage.Type {
	case LocalStorageType:
		RepoAvatar.Path = sec.Key("REPOSITORY_AVATAR_UPLOAD_PATH").MustString(filepath.Join(AppDataPath, "repo-avatars"))
		forcePathSeparator(RepoAvatar.Path)
		if !filepath.IsAbs(RepoAvatar.Path) {
			RepoAvatar.Path = filepath.Join(AppWorkPath, RepoAvatar.Path)
		}
	case MinioStorageType:
		RepoAvatar.Minio.Endpoint = sec.Key("REPOSITORY_AVATAR_MINIO_ENDPOINT").MustString(RepoAvatar.Minio.Endpoint)
		RepoAvatar.Minio.AccessKeyID = sec.Key("REPOSITORY_AVATAR_MINIO_ACCESS_KEY_ID").MustString(RepoAvatar.Minio.AccessKeyID)
		RepoAvatar.Minio.SecretAccessKey = sec.Key("REPOSITORY_AVATAR_MINIO_SECRET_ACCESS_KEY").MustString(RepoAvatar.Minio.SecretAccessKey)
		RepoAvatar.Minio.Bucket = sec.Key("REPOSITORY_AVATAR_MINIO_BUCKET").MustString(RepoAvatar.Minio.Bucket)
		RepoAvatar.Minio.Location = sec.Key("REPOSITORY_AVATAR_MINIO_LOCATION").MustString(RepoAvatar.Minio.Location)
		RepoAvatar.Minio.UseSSL = sec.Key("REPOSITORY_AVATAR_MINIO_USE_SSL").MustBool(RepoAvatar.Minio.UseSSL)
		RepoAvatar.Minio.BasePath = sec.Key("REPOSITORY_AVATAR_MINIO_BASE_PATH").MustString("avatars/")
	}

	RepoAvatar.Fallback = sec.Key("REPOSITORY_AVATAR_FALLBACK").MustString("none")
	RepoAvatar.FallbackImage = sec.Key("REPOSITORY_AVATAR_FALLBACK_IMAGE").MustString("/img/repo_default.png")
}

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
		StoreType   string
		UploadPath  string
		ServeDirect bool
		Minio       struct {
			Endpoint        string
			AccessKeyID     string
			SecretAccessKey string
			UseSSL          bool
			Bucket          string
			Location        string
			BasePath        string
		}
		MaxWidth    int
		MaxHeight   int
		MaxFileSize int64
	}{
		StoreType:   "local",
		ServeDirect: false,
		Minio: struct {
			Endpoint        string
			AccessKeyID     string
			SecretAccessKey string
			UseSSL          bool
			Bucket          string
			Location        string
			BasePath        string
		}{},
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
		StoreType   string
		UploadPath  string
		ServeDirect bool
		Minio       struct {
			Endpoint        string
			AccessKeyID     string
			SecretAccessKey string
			UseSSL          bool
			Bucket          string
			Location        string
			BasePath        string
		}
		Fallback      string
		FallbackImage string
	}{
		StoreType:   "local",
		ServeDirect: false,
	}
)

func newPictureService() {
	sec := Cfg.Section("picture")
	Avatar.StoreType = sec.Key("AVATAR_STORE_TYPE").MustString("local")
	Avatar.ServeDirect = sec.Key("AVATAR_SERVE_DIRECT").MustBool(false)
	switch Avatar.StoreType {
	case "local":
		Avatar.UploadPath = sec.Key("AVATAR_UPLOAD_PATH").MustString(filepath.Join(AppDataPath, "avatars"))
		forcePathSeparator(Avatar.UploadPath)
		if !filepath.IsAbs(Avatar.UploadPath) {
			Avatar.UploadPath = filepath.Join(AppWorkPath, Avatar.UploadPath)
		}
	case "minio":
		Avatar.Minio.Endpoint = sec.Key("AVATAR_MINIO_ENDPOINT").MustString("localhost:9000")
		Avatar.Minio.AccessKeyID = sec.Key("AVATAR_MINIO_ACCESS_KEY_ID").MustString("")
		Avatar.Minio.SecretAccessKey = sec.Key("AVATAR_MINIO_SECRET_ACCESS_KEY").MustString("")
		Avatar.Minio.Bucket = sec.Key("AVATAR_MINIO_BUCKET").MustString("gitea")
		Avatar.Minio.Location = sec.Key("AVATAR_MINIO_LOCATION").MustString("us-east-1")
		Avatar.Minio.BasePath = sec.Key("AVATAR_MINIO_BASE_PATH").MustString("avatars/")
		Avatar.Minio.UseSSL = sec.Key("AVATAR_MINIO_USE_SSL").MustBool(false)
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
	RepoAvatar.StoreType = sec.Key("REPOSITORY_AVATAR_STORE_TYPE").MustString("local")
	RepoAvatar.ServeDirect = sec.Key("REPOSITORY_AVATAR_SERVE_DIRECT").MustBool(false)
	switch RepoAvatar.StoreType {
	case "local":
		RepoAvatar.UploadPath = sec.Key("REPOSITORY_AVATAR_UPLOAD_PATH").MustString(filepath.Join(AppDataPath, "repo-avatars"))
		forcePathSeparator(RepoAvatar.UploadPath)
		if !filepath.IsAbs(RepoAvatar.UploadPath) {
			RepoAvatar.UploadPath = filepath.Join(AppWorkPath, RepoAvatar.UploadPath)
		}
	case "minio":
		RepoAvatar.Minio.Endpoint = sec.Key("REPOSITORY_AVATAR_MINIO_ENDPOINT").MustString("localhost:9000")
		RepoAvatar.Minio.AccessKeyID = sec.Key("REPOSITORY_AVATAR_MINIO_ACCESS_KEY_ID").MustString("")
		RepoAvatar.Minio.SecretAccessKey = sec.Key("REPOSITORY_AVATAR_MINIO_SECRET_ACCESS_KEY").MustString("")
		RepoAvatar.Minio.Bucket = sec.Key("REPOSITORY_AVATAR_MINIO_BUCKET").MustString("gitea")
		RepoAvatar.Minio.Location = sec.Key("REPOSITORY_AVATAR_MINIO_LOCATION").MustString("us-east-1")
		RepoAvatar.Minio.BasePath = sec.Key("REPOSITORY_AVATAR_MINIO_BASE_PATH").MustString("repo-avatars/")
		RepoAvatar.Minio.UseSSL = sec.Key("REPOSITORY_AVATAR_MINIO_USE_SSL").MustBool(false)
	}

	RepoAvatar.Fallback = sec.Key("REPOSITORY_AVATAR_FALLBACK").MustString("none")
	RepoAvatar.FallbackImage = sec.Key("REPOSITORY_AVATAR_FALLBACK_IMAGE").MustString("/img/repo_default.png")
}

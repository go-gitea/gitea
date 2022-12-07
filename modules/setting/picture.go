// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// settings
var (
	// Picture settings
	Avatar = struct {
		Storage

		MaxWidth           int
		MaxHeight          int
		MaxFileSize        int64
		RenderedSizeFactor int
	}{
		MaxWidth:           4096,
		MaxHeight:          3072,
		MaxFileSize:        1048576,
		RenderedSizeFactor: 3,
	}

	GravatarSource        string
	DisableGravatar       bool // Depreciated: migrated to database
	EnableFederatedAvatar bool // Depreciated: migrated to database

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
	Avatar.RenderedSizeFactor = sec.Key("AVATAR_RENDERED_SIZE_FACTOR").MustInt(3)

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

	DisableGravatar = sec.Key("DISABLE_GRAVATAR").MustBool(GetDefaultDisableGravatar())
	deprecatedSettingDB("", "DISABLE_GRAVATAR")
	EnableFederatedAvatar = sec.Key("ENABLE_FEDERATED_AVATAR").MustBool(GetDefaultEnableFederatedAvatar(DisableGravatar))
	deprecatedSettingDB("", "ENABLE_FEDERATED_AVATAR")

	newRepoAvatarService()
}

func GetDefaultDisableGravatar() bool {
	return !OfflineMode
}

func GetDefaultEnableFederatedAvatar(disableGravatar bool) bool {
	v := !InstallLock
	if OfflineMode {
		v = false
	}
	if disableGravatar {
		v = false
	}
	return v
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
	RepoAvatar.FallbackImage = sec.Key("REPOSITORY_AVATAR_FALLBACK_IMAGE").MustString("/assets/img/repo_default.png")
}

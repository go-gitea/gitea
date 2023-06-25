// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Avatar settings

var (
	Avatar = struct {
		Storage *Storage

		MaxWidth           int
		MaxHeight          int
		MaxFileSize        int64
		MaxOriginSize      int64
		RenderedSizeFactor int
	}{
		MaxWidth:           4096,
		MaxHeight:          4096,
		MaxFileSize:        1048576,
		MaxOriginSize:      262144,
		RenderedSizeFactor: 2,
	}

	GravatarSource        string
	DisableGravatar       bool // Depreciated: migrated to database
	EnableFederatedAvatar bool // Depreciated: migrated to database

	RepoAvatar = struct {
		Storage *Storage

		Fallback      string
		FallbackImage string
	}{}
)

func loadAvatarsFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("picture")

	avatarSec := rootCfg.Section("avatar")
	storageType := sec.Key("AVATAR_STORAGE_TYPE").MustString("")
	// Specifically default PATH to AVATAR_UPLOAD_PATH
	avatarSec.Key("PATH").MustString(sec.Key("AVATAR_UPLOAD_PATH").String())

	var err error
	Avatar.Storage, err = getStorage(rootCfg, "avatars", storageType, avatarSec)
	if err != nil {
		return err
	}

	Avatar.MaxWidth = sec.Key("AVATAR_MAX_WIDTH").MustInt(4096)
	Avatar.MaxHeight = sec.Key("AVATAR_MAX_HEIGHT").MustInt(4096)
	Avatar.MaxFileSize = sec.Key("AVATAR_MAX_FILE_SIZE").MustInt64(1048576)
	Avatar.MaxOriginSize = sec.Key("AVATAR_MAX_ORIGIN_SIZE").MustInt64(262144)
	Avatar.RenderedSizeFactor = sec.Key("AVATAR_RENDERED_SIZE_FACTOR").MustInt(2)

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
	deprecatedSettingDB(rootCfg, "", "DISABLE_GRAVATAR")
	EnableFederatedAvatar = sec.Key("ENABLE_FEDERATED_AVATAR").MustBool(GetDefaultEnableFederatedAvatar(DisableGravatar))
	deprecatedSettingDB(rootCfg, "", "ENABLE_FEDERATED_AVATAR")

	return nil
}

func GetDefaultDisableGravatar() bool {
	return OfflineMode
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

func loadRepoAvatarFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("picture")

	repoAvatarSec := rootCfg.Section("repo-avatar")
	storageType := sec.Key("REPOSITORY_AVATAR_STORAGE_TYPE").MustString("")
	// Specifically default PATH to AVATAR_UPLOAD_PATH
	repoAvatarSec.Key("PATH").MustString(sec.Key("REPOSITORY_AVATAR_UPLOAD_PATH").String())

	var err error
	RepoAvatar.Storage, err = getStorage(rootCfg, "repo-avatars", storageType, repoAvatarSec)
	if err != nil {
		return err
	}

	RepoAvatar.Fallback = sec.Key("REPOSITORY_AVATAR_FALLBACK").MustString("none")
	RepoAvatar.FallbackImage = sec.Key("REPOSITORY_AVATAR_FALLBACK_IMAGE").MustString(AppSubURL + "/assets/img/repo_default.png")

	return nil
}

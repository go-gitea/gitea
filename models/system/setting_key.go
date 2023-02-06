// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

// enumerate all system setting keys
const (
	KeyPictureDisableGravatar       = "picture.disable_gravatar"
	KeyPictureEnableFederatedAvatar = "picture.enable_federated_avatar"
)

// genSettingCacheKey returns the cache key for some configuration
func genSettingCacheKey(key string) string {
	return "system.setting." + key
}

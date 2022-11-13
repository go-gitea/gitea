// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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

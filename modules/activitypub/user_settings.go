// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	user_model "code.gitea.io/gitea/models/user"
)

// GetKeyPair function returns a user's private and public keys
func GetKeyPair(user *user_model.User) (pub, priv string, err error) {
	var settings map[string]*user_model.Setting
	settings, err = user_model.GetSettings(user.ID, []string{user_model.UserActivityPubPrivPem, user_model.UserActivityPubPubPem})
	if err != nil {
		return
	} else if len(settings) == 0 {
		if priv, pub, err = GenerateKeyPair(); err != nil {
			return
		}
		if err = user_model.SetUserSetting(user.ID, user_model.UserActivityPubPrivPem, priv); err != nil {
			return
		}
		if err = user_model.SetUserSetting(user.ID, user_model.UserActivityPubPubPem, pub); err != nil {
			return
		}
		return
	} else {
		priv = settings[user_model.UserActivityPubPrivPem].SettingValue
		pub = settings[user_model.UserActivityPubPubPem].SettingValue
		return
	}
}

// GetPublicKey function returns a user's public key
func GetPublicKey(user *user_model.User) (pub string, err error) {
	pub, _, err = GetKeyPair(user)
	return pub, err
}

// GetPrivateKey function returns a user's private key
func GetPrivateKey(user *user_model.User) (priv string, err error) {
	_, priv, err = GetKeyPair(user)
	return priv, err
}

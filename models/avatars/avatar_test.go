// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatars_test

import (
	"testing"

	avatars_model "code.gitea.io/gitea/models/avatars"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

const gravatarSource = "https://secure.gravatar.com/avatar/"

func disableGravatar(t *testing.T) {
	err := system_model.SetSettingNoVersion(system_model.KeyPictureEnableFederatedAvatar, "false")
	assert.NoError(t, err)
	err = system_model.SetSettingNoVersion(system_model.KeyPictureDisableGravatar, "true")
	assert.NoError(t, err)
	system_model.LibravatarService = nil
}

func enableGravatar(t *testing.T) {
	err := system_model.SetSettingNoVersion(system_model.KeyPictureDisableGravatar, "false")
	assert.NoError(t, err)
	setting.GravatarSource = gravatarSource
	err = system_model.Init()
	assert.NoError(t, err)
}

func TestHashEmail(t *testing.T) {
	assert.Equal(t,
		"d41d8cd98f00b204e9800998ecf8427e",
		avatars_model.HashEmail(""),
	)
	assert.Equal(t,
		"353cbad9b58e69c96154ad99f92bedc7",
		avatars_model.HashEmail("gitea@example.com"),
	)
}

func TestSizedAvatarLink(t *testing.T) {
	setting.AppSubURL = "/testsuburl"

	disableGravatar(t)
	assert.Equal(t, "/testsuburl/assets/img/avatar_default.png",
		avatars_model.GenerateEmailAvatarFastLink("gitea@example.com", 100))

	enableGravatar(t)
	assert.Equal(t,
		"https://secure.gravatar.com/avatar/353cbad9b58e69c96154ad99f92bedc7?d=identicon&s=100",
		avatars_model.GenerateEmailAvatarFastLink("gitea@example.com", 100),
	)
}

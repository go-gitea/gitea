// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package avatars

import (
	"net/url"
	"testing"

	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

const gravatarSource = "https://secure.gravatar.com/avatar/"

func disableGravatar() {
	system_model.SetSettingNoVersion("enable_federated_avatar", "false")
	system_model.SetSettingNoVersion("disable_gravatar", "true")
	system_model.LibravatarService = nil
}

func enableGravatar(t *testing.T) {
	system_model.SetSettingNoVersion("disable_gravatar", "false")
	var err error
	system_model.GravatarSourceURL, err = url.Parse(gravatarSource)
	assert.NoError(t, err)
}

func TestHashEmail(t *testing.T) {
	assert.Equal(t,
		"d41d8cd98f00b204e9800998ecf8427e",
		HashEmail(""),
	)
	assert.Equal(t,
		"353cbad9b58e69c96154ad99f92bedc7",
		HashEmail("gitea@example.com"),
	)
}

func TestSizedAvatarLink(t *testing.T) {
	setting.AppSubURL = "/testsuburl"

	disableGravatar()
	assert.Equal(t, "/testsuburl/assets/img/avatar_default.png",
		GenerateEmailAvatarFastLink("gitea@example.com", 100))

	enableGravatar(t)
	assert.Equal(t,
		"https://secure.gravatar.com/avatar/353cbad9b58e69c96154ad99f92bedc7?d=identicon&s=100",
		GenerateEmailAvatarFastLink("gitea@example.com", 100),
	)
}

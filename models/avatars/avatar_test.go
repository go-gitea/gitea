// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package avatars

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

const gravatarSource = "https://secure.gravatar.com/avatar/"

func disableGravatar() {
	setting.EnableFederatedAvatar = false
	setting.LibravatarService = nil
	setting.DisableGravatar = true
}

func enableGravatar(t *testing.T) {
	setting.DisableGravatar = false
	var err error
	setting.GravatarSourceURL, err = url.Parse(gravatarSource)
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

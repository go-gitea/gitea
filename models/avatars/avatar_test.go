// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatars_test

import (
	"strconv"
	"testing"

	avatars_model "gitea.dev/models/avatars"
	system_model "gitea.dev/models/system"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/setting/config"

	"github.com/stretchr/testify/assert"
)

func TestEmailAvatarLink(t *testing.T) {
	const email = "gitea@example.com"
	const emailHash = "72af1071d72449afe29e816060e78e78fd85829ba6e2497aa1d4eedd3c9dc611"
	setting.AppSubURL = "/testsuburl"
	setting.GravatarSource = "https://secure.gravatar.com/avatar/"

	assert.Equal(t, emailHash, avatars_model.HashEmail(" Gitea@Example.com "))

	setAvatarConfig := func(disableGravatar, enableFederatedAvatar bool) {
		assert.NoError(t, system_model.SetSettings(t.Context(), map[string]string{
			setting.Config().Picture.DisableGravatar.DynKey():       strconv.FormatBool(disableGravatar),
			setting.Config().Picture.EnableFederatedAvatar.DynKey(): strconv.FormatBool(enableFederatedAvatar),
		}))
		config.GetDynGetter().InvalidateCache()
	}

	setAvatarConfig(true, false)
	assert.Equal(t, "/testsuburl/assets/img/avatar_default.png",
		avatars_model.GenerateEmailAvatarFastLink(t.Context(), email, 100))

	setAvatarConfig(false, false)
	assert.Equal(t, "https://secure.gravatar.com/avatar/"+emailHash+"?d=identicon&s=100",
		avatars_model.GenerateEmailAvatarFastLink(t.Context(), email, 100))

	// the DNS query waits until the browser follows the link
	setAvatarConfig(false, true)
	assert.Equal(t, "/testsuburl/avatar/"+emailHash+"?size=100",
		avatars_model.GenerateEmailAvatarFastLink(t.Context(), email, 100))
	storedEmail, err := avatars_model.GetEmailForHash(t.Context(), emailHash)
	assert.NoError(t, err)
	assert.Equal(t, email, storedEmail)
	assert.Equal(t, "sha256", unittest.AssertExistsAndLoadBean(t, &avatars_model.EmailHash{Hash: emailHash}, unittest.OrderBy("hash")).HashType)
}

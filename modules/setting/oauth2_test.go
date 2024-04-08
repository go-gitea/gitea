// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestGetGeneralSigningSecret(t *testing.T) {
	// when there is no general signing secret, it should be generated, and keep the same value
	assert.Nil(t, generalSigningSecret.Load())
	s1 := GetGeneralTokenSigningSecret()
	assert.NotNil(t, s1)
	s2 := GetGeneralTokenSigningSecret()
	assert.Equal(t, s1, s2)

	// the config value should always override any pre-generated value
	cfg, _ := NewConfigProviderFromData(`
[oauth2]
JWT_SECRET = BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
`)
	defer test.MockVariableValue(&InstallLock, true)()
	loadOAuth2From(cfg)
	actual := GetGeneralTokenSigningSecret()
	expected, _ := generate.DecodeJwtSecretBase64("BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB")
	assert.Len(t, actual, 32)
	assert.EqualValues(t, expected, actual)
}

func TestOauth2DefaultApplications(t *testing.T) {
	cfg, _ := NewConfigProviderFromData(``)
	loadOAuth2From(cfg)
	assert.Equal(t, []string{"git-credential-oauth", "git-credential-manager", "tea"}, OAuth2.DefaultApplications)

	cfg, _ = NewConfigProviderFromData(`[oauth2]
DEFAULT_APPLICATIONS = tea
`)
	loadOAuth2From(cfg)
	assert.Equal(t, []string{"tea"}, OAuth2.DefaultApplications)

	cfg, _ = NewConfigProviderFromData(`[oauth2]
DEFAULT_APPLICATIONS =
`)
	loadOAuth2From(cfg)
	assert.Nil(t, nil, OAuth2.DefaultApplications)
}

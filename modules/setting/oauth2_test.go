// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

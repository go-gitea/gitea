// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"testing"

	"gitea.dev/modules/generate"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestGetGeneralSigningSecret(t *testing.T) {
	// when there is no general signing secret, it should be generated, and keep the same value
	generalSigningSecret.Store(nil)
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
	assert.Equal(t, expected, actual)
}

func TestGetGeneralSigningSecretSave(t *testing.T) {
	defer test.MockVariableValue(&InstallLock, true)()

	old := GetGeneralTokenSigningSecret()
	assert.Len(t, old, 32)

	tmpFile := t.TempDir() + "/app.ini"
	_ = os.WriteFile(tmpFile, nil, 0o644)
	cfg, _ := NewConfigProviderFromFile(tmpFile)
	loadOAuth2From(cfg)
	generated := GetGeneralTokenSigningSecret()
	assert.Len(t, generated, 32)
	assert.NotEqual(t, old, generated)

	generalSigningSecret.Store(nil)
	cfg, _ = NewConfigProviderFromFile(tmpFile)
	loadOAuth2From(cfg)
	again := GetGeneralTokenSigningSecret()
	assert.Equal(t, generated, again)

	iniContent, err := os.ReadFile(tmpFile)
	assert.NoError(t, err)
	assert.Contains(t, string(iniContent), "JWT_SECRET = ")
}

func TestOauth2DefaultApplications(t *testing.T) {
	defer test.MockVariableValue(&OAuth2)()

	testConfigLoad(t, []any{loadOAuth2From}, []configTestCase{
		{
			name: "defaults",
			want: []configCheck{field("DEFAULT_APPLICATIONS", &OAuth2.DefaultApplications, []string{"git-credential-oauth", "git-credential-manager", "tea"})},
		},
		{
			name: "a single application",
			ini:  "[oauth2]\nDEFAULT_APPLICATIONS = tea",
			want: []configCheck{field("DEFAULT_APPLICATIONS", &OAuth2.DefaultApplications, []string{"tea"})},
		},
		{
			name: "an empty value disables them all",
			ini:  "[oauth2]\nDEFAULT_APPLICATIONS =",
			want: []configCheck{field("DEFAULT_APPLICATIONS", &OAuth2.DefaultApplications, []string(nil))},
		},
	})
}

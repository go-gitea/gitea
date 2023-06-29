// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getStorageInheritNameSectionTypeForActions(t *testing.T) {
	iniStr := `
	[storage]
	STORAGE_TYPE = minio
	`
	cfg, err := NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.MinioConfig.BasePath)
	assert.EqualValues(t, "minio", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts/", Actions.ArtifactStorage.MinioConfig.BasePath)

	iniStr = `
[storage.actions_log]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.MinioConfig.BasePath)
	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts", filepath.Base(Actions.ArtifactStorage.Path))

	iniStr = `
[storage.actions_log]
STORAGE_TYPE = my_storage

[storage.my_storage]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "minio", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log/", Actions.LogStorage.MinioConfig.BasePath)
	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts", filepath.Base(Actions.ArtifactStorage.Path))

	iniStr = `
[storage.actions_artifacts]
STORAGE_TYPE = my_storage

[storage.my_storage]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log", filepath.Base(Actions.LogStorage.Path))
	assert.EqualValues(t, "minio", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts/", Actions.ArtifactStorage.MinioConfig.BasePath)

	iniStr = `
[storage.actions_artifacts]
STORAGE_TYPE = my_storage

[storage.my_storage]
STORAGE_TYPE = minio
`
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log", filepath.Base(Actions.LogStorage.Path))
	assert.EqualValues(t, "minio", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts/", Actions.ArtifactStorage.MinioConfig.BasePath)

	iniStr = ``
	cfg, err = NewConfigProviderFromData(iniStr)
	assert.NoError(t, err)
	assert.NoError(t, loadActionsFrom(cfg))

	assert.EqualValues(t, "local", Actions.LogStorage.Type)
	assert.EqualValues(t, "actions_log", filepath.Base(Actions.LogStorage.Path))
	assert.EqualValues(t, "local", Actions.ArtifactStorage.Type)
	assert.EqualValues(t, "actions_artifacts", filepath.Base(Actions.ArtifactStorage.Path))
}

func Test_getDefaultActionsURLForActions(t *testing.T) {
	oldActions := Actions
	oldAppURL := AppURL
	defer func() {
		Actions = oldActions
		AppURL = oldAppURL
	}()

	AppURL = "http://test_get_default_actions_url_for_actions:3000/"

	tests := []struct {
		name    string
		iniStr  string
		wantErr assert.ErrorAssertionFunc
		wantURL string
	}{
		{
			name: "default",
			iniStr: `
[actions]
`,
			wantErr: assert.NoError,
			wantURL: "https://github.com",
		},
		{
			name: "github",
			iniStr: `
[actions]
DEFAULT_ACTIONS_URL = github
`,
			wantErr: assert.NoError,
			wantURL: "https://github.com",
		},
		{
			name: "self",
			iniStr: `
[actions]
DEFAULT_ACTIONS_URL = self
`,
			wantErr: assert.NoError,
			wantURL: "http://test_get_default_actions_url_for_actions:3000",
		},
		{
			name: "custom url",
			iniStr: `
[actions]
DEFAULT_ACTIONS_URL = https://gitea.com
`,
			wantErr: assert.NoError,
			wantURL: "https://github.com",
		},
		{
			name: "custom urls",
			iniStr: `
[actions]
DEFAULT_ACTIONS_URL = https://gitea.com,https://github.com
`,
			wantErr: assert.NoError,
			wantURL: "https://github.com",
		},
		{
			name: "invalid",
			iniStr: `
[actions]
DEFAULT_ACTIONS_URL = gitea
`,
			wantErr: assert.Error,
			wantURL: "https://github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := NewConfigProviderFromData(tt.iniStr)
			require.NoError(t, err)
			if !tt.wantErr(t, loadActionsFrom(cfg)) {
				return
			}
			assert.EqualValues(t, tt.wantURL, Actions.DefaultActionsURL.URL())
		})
	}
}

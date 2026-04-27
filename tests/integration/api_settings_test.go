// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIExposedSettings(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	req := NewRequest(t, "GET", "/api/v1/settings/ui")
	resp := MakeRequest(t, req, http.StatusOK)

	ui := DecodeJSON(t, resp, &api.GeneralUISettings{})
	assert.Len(t, ui.AllowedReactions, len(setting.UI.Reactions))
	assert.ElementsMatch(t, setting.UI.Reactions, ui.AllowedReactions)

	req = NewRequest(t, "GET", "/api/v1/settings/api")
	resp = MakeRequest(t, req, http.StatusOK)

	apiSettings := DecodeJSON(t, resp, &api.GeneralAPISettings{})
	assert.Equal(t, &api.GeneralAPISettings{
		MaxResponseItems:       setting.API.MaxResponseItems,
		DefaultPagingNum:       setting.API.DefaultPagingNum,
		DefaultGitTreesPerPage: setting.API.DefaultGitTreesPerPage,
		DefaultMaxBlobSize:     setting.API.DefaultMaxBlobSize,
		DefaultMaxResponseSize: setting.API.DefaultMaxResponseSize,
	}, apiSettings)

	req = NewRequest(t, "GET", "/api/v1/settings/repository")
	resp = MakeRequest(t, req, http.StatusOK)

	repo := DecodeJSON(t, resp, &api.GeneralRepoSettings{})
	assert.Equal(t, &api.GeneralRepoSettings{
		MirrorsDisabled:      !setting.Mirror.Enabled,
		HTTPGitDisabled:      setting.Repository.DisableHTTPGit,
		MigrationsDisabled:   setting.Repository.DisableMigrations,
		TimeTrackingDisabled: false,
		LFSDisabled:          !setting.LFS.StartServer,
	}, repo)

	req = NewRequest(t, "GET", "/api/v1/settings/attachment")
	resp = MakeRequest(t, req, http.StatusOK)

	attachment := DecodeJSON(t, resp, &api.GeneralAttachmentSettings{})
	assert.Equal(t, &api.GeneralAttachmentSettings{
		Enabled:      setting.Attachment.Enabled,
		AllowedTypes: setting.Attachment.AllowedTypes,
		MaxFiles:     setting.Attachment.MaxFiles,
		MaxSize:      setting.Attachment.MaxSize,
	}, attachment)
}

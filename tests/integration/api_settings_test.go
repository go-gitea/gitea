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

	ui := new(api.GeneralUISettings)
	req := NewRequest(t, "GET", "/api/v1/settings/ui")
	resp := MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &ui)
	assert.Len(t, ui.AllowedReactions, len(setting.UI.Reactions))
	assert.ElementsMatch(t, setting.UI.Reactions, ui.AllowedReactions)

	apiSettings := new(api.GeneralAPISettings)
	req = NewRequest(t, "GET", "/api/v1/settings/api")
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &apiSettings)
	assert.EqualValues(t, &api.GeneralAPISettings{
		MaxResponseItems:       setting.API.MaxResponseItems,
		DefaultPagingNum:       setting.API.DefaultPagingNum,
		DefaultGitTreesPerPage: setting.API.DefaultGitTreesPerPage,
		DefaultMaxBlobSize:     setting.API.DefaultMaxBlobSize,
	}, apiSettings)

	repo := new(api.GeneralRepoSettings)
	req = NewRequest(t, "GET", "/api/v1/settings/repository")
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &repo)
	assert.EqualValues(t, &api.GeneralRepoSettings{
		MirrorsDisabled:      !setting.Mirror.Enabled,
		HTTPGitDisabled:      setting.Repository.DisableHTTPGit,
		MigrationsDisabled:   setting.Repository.DisableMigrations,
		TimeTrackingDisabled: false,
		LFSDisabled:          !setting.LFS.StartServer,
	}, repo)

	attachment := new(api.GeneralAttachmentSettings)
	req = NewRequest(t, "GET", "/api/v1/settings/attachment")
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &attachment)
	assert.EqualValues(t, &api.GeneralAttachmentSettings{
		Enabled:      setting.Attachment.Enabled,
		AllowedTypes: setting.Attachment.AllowedTypes,
		MaxFiles:     setting.Attachment.MaxFiles,
		MaxSize:      setting.Attachment.MaxSize,
	}, attachment)
}

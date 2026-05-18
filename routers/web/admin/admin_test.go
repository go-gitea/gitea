// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfCheckPost(t *testing.T) {
	defer test.MockVariableValue(&setting.PublicURLDetection)()
	defer test.MockVariableValue(&setting.AppURL, "http://config/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()

	data := struct {
		Problems []string `json:"problems"`
	}{}

	setting.PublicURLDetection = setting.PublicURLLegacy
	ctx, resp := contexttest.MockContext(t, "GET http://host/sub/admin/self_check?location_origin=http://frontend")
	SelfCheckPost(ctx)
	assert.Equal(t, http.StatusOK, resp.Code)
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &data))
	assert.Equal(t, []string{
		ctx.Locale.TrString("admin.self_check.location_origin_mismatch", "http://frontend/sub/", "http://config/sub/"),
	}, data.Problems)

	setting.PublicURLDetection = setting.PublicURLAuto
	ctx, resp = contexttest.MockContext(t, "GET http://host/sub/admin/self_check?location_origin=http://frontend")
	SelfCheckPost(ctx)
	assert.Equal(t, http.StatusOK, resp.Code)
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &data))
	assert.Equal(t, []string{
		ctx.Locale.TrString("admin.self_check.location_origin_mismatch", "http://frontend/sub/", "http://host/sub/"),
	}, data.Problems)
}

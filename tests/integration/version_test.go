// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.AppVer = "test-version-1"
	req := NewRequest(t, "GET", "/api/v1/version")
	resp := MakeRequest(t, req, http.StatusOK)

	var version structs.ServerVersion
	DecodeJSON(t, resp, &version)
	assert.Equal(t, setting.AppVer, version.Version)
}

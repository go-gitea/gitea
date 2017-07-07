// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func assertProtectedBranch(t *testing.T, repoID int64, branchName string, isErr, canPush bool) {
	reqURL := fmt.Sprintf("/api/internal/branch/%d/%s", repoID, url.QueryEscape(branchName))
	req := NewRequest(t, "GET", reqURL)
	t.Log(reqURL)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", setting.InternalToken))

	resp := MakeRequest(t, req, NoExpectedStatus)
	if isErr {
		assert.EqualValues(t, http.StatusInternalServerError, resp.HeaderCode)
	} else {
		assert.EqualValues(t, http.StatusOK, resp.HeaderCode)
		var branch models.ProtectedBranch
		t.Log(string(resp.Body))
		assert.NoError(t, json.Unmarshal(resp.Body, &branch))
		assert.Equal(t, canPush, branch.CanPush)
	}
}

func TestInternal_GetProtectedBranch(t *testing.T) {
	prepareTestEnv(t)

	assertProtectedBranch(t, 1, "master", false, true)
	assertProtectedBranch(t, 1, "dev", false, true)
	assertProtectedBranch(t, 1, "lunny/dev", false, true)
}

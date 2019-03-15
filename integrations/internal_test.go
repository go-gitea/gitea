// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func assertProtectedBranch(t *testing.T, repoID int64, branchName string, isErr, canPush bool) {
	reqURL := fmt.Sprintf("/api/internal/branch/%d/%s", repoID, util.PathEscapeSegments(branchName))
	req := NewRequest(t, "GET", reqURL)
	t.Log(reqURL)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", setting.InternalToken))

	resp := MakeRequest(t, req, NoExpectedStatus)
	if isErr {
		assert.EqualValues(t, http.StatusInternalServerError, resp.Code)
	} else {
		assert.EqualValues(t, http.StatusOK, resp.Code)
		var branch models.ProtectedBranch
		t.Log(resp.Body.String())
		assert.NoError(t, json.Unmarshal(resp.Body.Bytes(), &branch))
		assert.Equal(t, canPush, !branch.IsProtected())
	}
}

func TestInternal_GetProtectedBranch(t *testing.T) {
	prepareTestEnv(t)

	assertProtectedBranch(t, 1, "master", false, true)
	assertProtectedBranch(t, 1, "dev", false, true)
	assertProtectedBranch(t, 1, "lunny/dev", false, true)
}

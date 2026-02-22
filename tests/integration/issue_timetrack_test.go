// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestIssueTimeDeleteScoped(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.NoError(t, issue1.LoadRepo(t.Context()))
	tracked := unittest.AssertExistsAndLoadBean(t, &issues_model.TrackedTime{ID: 5})

	session := loginUser(t, issue1.Repo.OwnerName)
	url := fmt.Sprintf("/%s/%s/issues/%d/times/%d/delete", issue1.Repo.OwnerName, issue1.Repo.Name, issue1.Index, tracked.ID)
	req := NewRequestWithValues(t, "POST", url, map[string]string{})
	session.MakeRequest(t, req, http.StatusNotFound)

	tracked = unittest.AssertExistsAndLoadBean(t, &issues_model.TrackedTime{ID: tracked.ID})
	assert.False(t, tracked.Deleted)
}

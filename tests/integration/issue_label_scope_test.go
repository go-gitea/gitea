// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/tests"
)

// TestUpdateIssueLabelForeignLabel verifies that the web issue-label action rejects a
// label owned by a different repo/org with 404 (indistinguishable from a nonexistent
// id), closing the cross-repo label enumeration oracle also on the web side.
func TestUpdateIssueLabelForeignLabel(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	sess := loginUser(t, "user2")

	// label 3 belongs to org3 — foreign to user2/repo1 (issue 1); must be 404, not a 500 oracle
	req := NewRequestWithValues(t, "POST", "/user2/repo1/issues/labels?issue_ids=1", map[string]string{
		"action": "attach",
		"id":     "3",
	})
	sess.MakeRequest(t, req, http.StatusNotFound)

	// a label owned by the repo still attaches normally
	req = NewRequestWithValues(t, "POST", "/user2/repo1/issues/labels?issue_ids=1", map[string]string{
		"action": "attach",
		"id":     "2",
	})
	sess.MakeRequest(t, req, http.StatusOK)
}

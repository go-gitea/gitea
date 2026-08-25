// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateIssueAttachmentsCrossRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// attachment id 2 belongs to repo 2 / issue 4; issue 1 lives in repo 1
	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	foreign := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 2})
	require.NotEqual(t, issue1.RepoID, foreign.RepoID)

	// re-linking a foreign repo's attachment by UUID must be rejected
	err := issues_model.UpdateIssueAttachments(t.Context(), issue1.ID, []string{foreign.UUID})
	assert.ErrorIs(t, err, util.ErrPermissionDenied)

	// the foreign attachment must be left untouched
	reloaded := unittest.AssertExistsAndLoadBean(t, &repo_model.Attachment{ID: 2})
	assert.Equal(t, foreign.IssueID, reloaded.IssueID)
}

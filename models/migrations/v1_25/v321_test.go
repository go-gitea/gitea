// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_AddCombinedIndexToIssueUser(t *testing.T) {
	type Comment struct { // old struct
		ID               int64 `xorm:"pk autoincr"`
		Type             int   `xorm:"INDEX"`
		PosterID         int64 `xorm:"INDEX"`
		OriginalAuthor   string
		OriginalAuthorID int64
		IssueID          int64 `xorm:"INDEX"`
		LabelID          int64
		OldProjectID     int64
		ProjectID        int64
		OldMilestoneID   int64
		MilestoneID      int64
		TimeID           int64
		AssigneeID       int64
		RemovedAssignee  bool
		AssigneeTeamID   int64 `xorm:"NOT NULL DEFAULT 0"`
		ResolveDoerID    int64
		OldTitle         string
		NewTitle         string
		OldRef           string
		NewRef           string
		DependentIssueID int64 `xorm:"index"` // This is used by issue_service.deleteIssue

		CommitID       int64
		Line           int64 // - previous line / + proposed line
		TreePath       string
		Content        string `xorm:"LONGTEXT"`
		ContentVersion int    `xorm:"NOT NULL DEFAULT 0"`

		// Path represents the 4 lines of code cemented by this comment
		Patch       string `xorm:"-"`
		PatchQuoted string `xorm:"LONGTEXT patch"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

		// Reference issue in commit message
		CommitSHA string `xorm:"VARCHAR(64)"`

		ReviewID    int64 `xorm:"index"`
		Invalidated bool

		// Reference an issue or pull from another comment, issue or PR
		// All information is about the origin of the reference
		RefRepoID    int64                 `xorm:"index"` // Repo where the referencing
		RefIssueID   int64                 `xorm:"index"`
		RefCommentID int64                 `xorm:"index"`    // 0 if origin is Issue title or content (or PR's)
		RefAction    references.XRefAction `xorm:"SMALLINT"` // What happens if RefIssueID resolves
		RefIsPull    bool

		CommentMetaData string `xorm:"JSON TEXT"` // put all non-index metadata in a single field
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(Comment))
	defer deferable()

	assert.NoError(t, AddBeforeCommitIDForComment(x))
}

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Normalize
// ---------------------------------------------------------------------------

func TestCloseOptions_Normalize(t *testing.T) {
	t.Run("empty reason becomes completed", func(t *testing.T) {
		o := CloseOptions{}
		o.Normalize()
		assert.Equal(t, CloseReasonCompleted, o.Reason)
	})

	t.Run("non-empty reason is unchanged", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonNotPlanned}
		o.Normalize()
		assert.Equal(t, CloseReasonNotPlanned, o.Reason)
	})

	t.Run("system reason is unchanged", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonCompletedByCommit}
		o.Normalize()
		assert.Equal(t, CloseReasonCompletedByCommit, o.Reason)
	})
}

// ---------------------------------------------------------------------------
// IsSystemOnly
// ---------------------------------------------------------------------------

func TestCloseOptions_IsSystemOnly(t *testing.T) {
	cases := []struct {
		reason   issues_model.IssueCloseReason
		expected bool
	}{
		{CloseReasonCompleted, false},
		{CloseReasonNotPlanned, false},
		{CloseReasonDuplicate, false},
		{CloseReasonAnswered, false},
		{CloseReasonCompletedByCommit, true},
		{CloseReasonCompletedByPull, true},
	}
	for _, c := range cases {
		o := CloseOptions{Reason: c.reason}
		assert.Equal(t, c.expected, o.IsSystemOnly(), "reason=%s", c.reason)
	}
}

// ---------------------------------------------------------------------------
// Constructor helpers
// ---------------------------------------------------------------------------

func TestCloseOptionsConstructors(t *testing.T) {
	t.Run("CloseOptionsCompleted", func(t *testing.T) {
		o := CloseOptionsCompleted()
		assert.Equal(t, CloseReasonCompleted, o.Reason)
		assert.Empty(t, o.ReasonParam)
	})

	t.Run("CloseOptionsNotPlanned", func(t *testing.T) {
		o := CloseOptionsNotPlanned()
		assert.Equal(t, CloseReasonNotPlanned, o.Reason)
		assert.Empty(t, o.ReasonParam)
	})

	t.Run("CloseOptionsDuplicate", func(t *testing.T) {
		o := CloseOptionsDuplicate(42)
		assert.Equal(t, CloseReasonDuplicate, o.Reason)
		var p CloseReasonDuplicateParam
		require.NoError(t, json.Unmarshal([]byte(o.ReasonParam), &p))
		assert.Equal(t, int64(42), p.IssueIndex)
	})

	t.Run("CloseOptionsAnswered", func(t *testing.T) {
		o := CloseOptionsAnswered(99)
		assert.Equal(t, CloseReasonAnswered, o.Reason)
		var p CloseReasonAnsweredParam
		require.NoError(t, json.Unmarshal([]byte(o.ReasonParam), &p))
		assert.Equal(t, int64(99), p.CommentID)
	})

	t.Run("CloseOptionsCompletedByCommit", func(t *testing.T) {
		o := CloseOptionsCompletedByCommit("deadbeef")
		assert.Equal(t, CloseReasonCompletedByCommit, o.Reason)
		var p CloseReasonCommitParam
		require.NoError(t, json.Unmarshal([]byte(o.ReasonParam), &p))
		assert.Equal(t, "deadbeef", p.CommitHash)
	})

	t.Run("CloseOptionsCompletedByPull", func(t *testing.T) {
		o := CloseOptionsCompletedByPull(7)
		assert.Equal(t, CloseReasonCompletedByPull, o.Reason)
		var p CloseReasonPullParam
		require.NoError(t, json.Unmarshal([]byte(o.ReasonParam), &p))
		assert.Equal(t, int64(7), p.PullIndex)
	})
}

// ---------------------------------------------------------------------------
// Validate — no-DB cases
// ---------------------------------------------------------------------------

var issueForValidate = &issues_model.Issue{ID: 1, RepoID: 1, Index: 1}

func TestValidate_NoParam_Reasons(t *testing.T) {
	for _, reason := range []issues_model.IssueCloseReason{CloseReasonCompleted, CloseReasonNotPlanned} {
		o := CloseOptions{Reason: reason}
		assert.NoError(t, o.Validate(t.Context(), issueForValidate), "reason=%s", reason.String())
	}
}

func TestValidate_UnknownReason(t *testing.T) {
	o := CloseOptions{Reason: issues_model.IssueCloseReason(999)}
	err := o.Validate(t.Context(), issueForValidate)
	require.Error(t, err)
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
}

func TestValidate_CompletedByCommit(t *testing.T) {
	t.Run("valid hash", func(t *testing.T) {
		o := CloseOptionsCompletedByCommit("abc123")
		assert.NoError(t, o.Validate(t.Context(), issueForValidate))
	})

	t.Run("empty hash is rejected", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonCompletedByCommit, ReasonParam: `{"commit_hash":""}`}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("missing param treated as zero-value", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonCompletedByCommit, ReasonParam: ""}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("invalid JSON param", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonCompletedByCommit, ReasonParam: "not-json"}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}

func TestValidate_CompletedByPull(t *testing.T) {
	t.Run("valid pull index", func(t *testing.T) {
		o := CloseOptionsCompletedByPull(3)
		assert.NoError(t, o.Validate(t.Context(), issueForValidate))
	})

	t.Run("zero index is rejected", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonCompletedByPull, ReasonParam: `{"pull_index":0}`}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("missing param treated as zero-value", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonCompletedByPull, ReasonParam: ""}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}

func TestValidate_Duplicate_NoDBErrors(t *testing.T) {
	t.Run("zero index is rejected without DB call", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonDuplicate, ReasonParam: `{"issue_index":0}`}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("self-reference is rejected without DB call", func(t *testing.T) {
		// issueForValidate has Index=1; duplicate param also 1
		o := CloseOptionsDuplicate(1)
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("missing param treated as zero-value", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonDuplicate, ReasonParam: ""}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("invalid JSON param", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonDuplicate, ReasonParam: "not-json"}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}

func TestValidate_Answered_NoDBErrors(t *testing.T) {
	t.Run("zero comment id is rejected without DB call", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonAnswered, ReasonParam: `{"comment_id":0}`}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("missing param treated as zero-value", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonAnswered, ReasonParam: ""}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("invalid JSON param", func(t *testing.T) {
		o := CloseOptions{Reason: CloseReasonAnswered, ReasonParam: "not-json"}
		err := o.Validate(t.Context(), issueForValidate)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}

// ---------------------------------------------------------------------------
// Validate — DB-dependent cases
// Fixtures used:
//   issue (repo_id=1, id=1, index=1, is_pull=false)  ← "current issue"
//   issue (repo_id=1, id=5, index=4, is_pull=false)  ← valid duplicate target
//   issue (repo_id=1, id=2, index=2, is_pull=true)   ← PR, must be rejected
//   comment (id=2, type=0, issue_id=1)                ← valid answered comment
//   comment (id=4, type=21, issue_id=2)               ← belongs to different issue
// ---------------------------------------------------------------------------

func TestValidate_Duplicate_DB(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Reload current issue from DB so all fields are populated.
	currentIssue, err := issues_model.GetIssueByIndex(t.Context(), 1, 1)
	require.NoError(t, err)

	t.Run("valid non-PR same-repo issue", func(t *testing.T) {
		// index=4 in repo_id=1 is a regular issue (id=5)
		o := CloseOptionsDuplicate(4)
		assert.NoError(t, o.Validate(t.Context(), currentIssue))
	})

	t.Run("target is a pull request", func(t *testing.T) {
		// index=2 in repo_id=1 is is_pull=true
		o := CloseOptionsDuplicate(2)
		err := o.Validate(t.Context(), currentIssue)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("target issue does not exist", func(t *testing.T) {
		o := CloseOptionsDuplicate(99999)
		err := o.Validate(t.Context(), currentIssue)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}

func TestValidate_Answered_DB(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	currentIssue, err := issues_model.GetIssueByIndex(t.Context(), 1, 1)
	require.NoError(t, err)

	t.Run("valid comment belonging to this issue", func(t *testing.T) {
		// comment id=2 has issue_id=1
		o := CloseOptionsAnswered(2)
		assert.NoError(t, o.Validate(t.Context(), currentIssue))
	})

	t.Run("comment belongs to a different issue", func(t *testing.T) {
		// comment id=4 has issue_id=2
		o := CloseOptionsAnswered(4)
		err := o.Validate(t.Context(), currentIssue)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})

	t.Run("comment does not exist", func(t *testing.T) {
		o := CloseOptionsAnswered(99999)
		err := o.Validate(t.Context(), currentIssue)
		require.Error(t, err)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}

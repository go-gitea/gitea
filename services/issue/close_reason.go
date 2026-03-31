// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

const (
	CloseReasonNone              = issues_model.IssueCloseReasonNone
	CloseReasonCompleted         = issues_model.IssueCloseReasonCompleted
	CloseReasonCompletedByCommit = issues_model.IssueCloseReasonCompletedByCommit
	CloseReasonCompletedByPull   = issues_model.IssueCloseReasonCompletedByPull
	CloseReasonAnswered          = issues_model.IssueCloseReasonAnswered
	CloseReasonDuplicate         = issues_model.IssueCloseReasonDuplicate
	CloseReasonNotPlanned        = issues_model.IssueCloseReasonNotPlanned
)

// systemOnlyCloseReasons maps reasons that must not be written by external
// API or Web requests directly from users.
var systemOnlyCloseReasons = map[issues_model.IssueCloseReason]bool{
	CloseReasonCompletedByCommit: true,
	CloseReasonCompletedByPull:   true,
}

// CloseReasonDuplicateParam is the JSON param for the "duplicate" close reason.
type CloseReasonDuplicateParam struct {
	IssueIndex int64 `json:"issue_index"` // same-repo, non-PR issue index
}

// CloseReasonAnsweredParam is the JSON param for the "answered" close reason.
type CloseReasonAnsweredParam struct {
	CommentID int64 `json:"comment_id"` // comment ID belonging to the current issue
}

// CloseReasonCommitParam is the JSON param for the "completed_by_commit" close reason.
type CloseReasonCommitParam struct {
	CommitHash string `json:"commit_hash"` // full or abbreviated commit hash that triggered the close
}

// CloseReasonPullParam is the JSON param for the "completed_by_pull" close reason.
type CloseReasonPullParam struct {
	PullIndex int64 `json:"pull_index"` // pull request index that triggered the close
}

// CloseOptions carries the close reason and its serialized param.
// ReasonParam is a JSON string whose schema depends on Reason:
//   - completed / not_planned: empty
//   - duplicate: CloseReasonDuplicateParam
//   - answered: CloseReasonAnsweredParam
//   - completed_by_commit: CloseReasonCommitParam
//   - completed_by_pull: CloseReasonPullParam
type CloseOptions struct {
	Reason      issues_model.IssueCloseReason
	ReasonParam string // JSON-serialized param, empty when no param is needed
}

// IsSystemOnly returns true when this reason must only be written by internal
// code and must not be accepted from external API or Web requests.
func (o CloseOptions) IsSystemOnly() bool {
	return systemOnlyCloseReasons[o.Reason]
}

// Normalize fills in the default close reason ("completed") when Reason is empty.
func (o *CloseOptions) Normalize() {
	if o.Reason == CloseReasonNone {
		o.Reason = CloseReasonCompleted
	}
}

// Validate checks that Reason is known, that the serialized param is valid for
// the given reason, and that any referenced issue or comment actually exists
// in the repository and satisfies the required constraints.
func (o *CloseOptions) Validate(ctx context.Context, issue *issues_model.Issue) error {
	switch o.Reason {
	case CloseReasonCompleted, CloseReasonNotPlanned:
		// no param required

	case CloseReasonDuplicate:
		var p CloseReasonDuplicateParam
		if err := unmarshalParam(o.ReasonParam, &p); err != nil {
			return util.NewInvalidArgumentErrorf("duplicate close reason param is invalid: %v", err)
		}
		if p.IssueIndex <= 0 {
			return util.NewInvalidArgumentErrorf("duplicate close reason requires a valid issue_index")
		}
		if p.IssueIndex == issue.Index {
			return util.NewInvalidArgumentErrorf("duplicate close reason cannot reference the current issue itself")
		}
		target, err := issues_model.GetIssueByIndex(ctx, issue.RepoID, p.IssueIndex)
		if err != nil {
			if issues_model.IsErrIssueNotExist(err) {
				return util.NewInvalidArgumentErrorf("duplicate target issue #%d does not exist in this repository", p.IssueIndex)
			}
			return err
		}
		if target.IsPull {
			return util.NewInvalidArgumentErrorf("duplicate close reason cannot reference a pull request (#%d)", p.IssueIndex)
		}

	case CloseReasonAnswered:
		var p CloseReasonAnsweredParam
		if err := unmarshalParam(o.ReasonParam, &p); err != nil {
			return util.NewInvalidArgumentErrorf("answered close reason param is invalid: %v", err)
		}
		if p.CommentID <= 0 {
			return util.NewInvalidArgumentErrorf("answered close reason requires a valid comment_id")
		}
		comment, err := issues_model.GetCommentByID(ctx, p.CommentID)
		if err != nil {
			if issues_model.IsErrCommentNotExist(err) {
				return util.NewInvalidArgumentErrorf("answered comment #%d does not exist", p.CommentID)
			}
			return err
		}
		if comment.IssueID != issue.ID {
			return util.NewInvalidArgumentErrorf("answered comment #%d does not belong to this issue", p.CommentID)
		}

	case CloseReasonCompletedByCommit:
		var p CloseReasonCommitParam
		if err := unmarshalParam(o.ReasonParam, &p); err != nil {
			return util.NewInvalidArgumentErrorf("completed_by_commit param is invalid: %v", err)
		}
		if p.CommitHash == "" {
			return util.NewInvalidArgumentErrorf("completed_by_commit requires a non-empty commit_hash")
		}

	case CloseReasonCompletedByPull:
		var p CloseReasonPullParam
		if err := unmarshalParam(o.ReasonParam, &p); err != nil {
			return util.NewInvalidArgumentErrorf("completed_by_pull param is invalid: %v", err)
		}
		if p.PullIndex <= 0 {
			return util.NewInvalidArgumentErrorf("completed_by_pull requires a valid pull_index")
		}

	default:
		return util.NewInvalidArgumentErrorf("unknown close reason %d", o.Reason)
	}
	return nil
}

// unmarshalParam is a small helper that rejects an empty param string for
// reasons that actually require a param (callers catch the zero-value case
// themselves after unmarshalling).
func unmarshalParam(param string, dst any) error {
	if param == "" {
		// Unmarshal into zero-value struct; callers validate individual fields.
		return nil
	}
	return json.Unmarshal([]byte(param), dst)
}

// Constructor helpers — use these instead of building CloseOptions by hand.

func CloseOptionsCompleted() CloseOptions {
	return CloseOptions{Reason: CloseReasonCompleted}
}

func CloseOptionsNotPlanned() CloseOptions {
	return CloseOptions{Reason: CloseReasonNotPlanned}
}

func CloseOptionsDuplicate(issueIndex int64) CloseOptions {
	b, _ := json.Marshal(CloseReasonDuplicateParam{IssueIndex: issueIndex})
	return CloseOptions{Reason: CloseReasonDuplicate, ReasonParam: string(b)}
}

func CloseOptionsAnswered(commentID int64) CloseOptions {
	b, _ := json.Marshal(CloseReasonAnsweredParam{CommentID: commentID})
	return CloseOptions{Reason: CloseReasonAnswered, ReasonParam: string(b)}
}

func CloseOptionsCompletedByCommit(commitHash string) CloseOptions {
	b, _ := json.Marshal(CloseReasonCommitParam{CommitHash: commitHash})
	return CloseOptions{Reason: CloseReasonCompletedByCommit, ReasonParam: string(b)}
}

func CloseOptionsCompletedByPull(pullIndex int64) CloseOptions {
	b, _ := json.Marshal(CloseReasonPullParam{PullIndex: pullIndex})
	return CloseOptions{Reason: CloseReasonCompletedByPull, ReasonParam: string(b)}
}

// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"gitea.dev/models/db"
)

// MergeStyle represents the approach to merge commits into base branch.
type MergeStyle string

const (
	// MergeStyleMerge create merge commit
	MergeStyleMerge MergeStyle = "merge"
	// MergeStyleRebase rebase before merging, and fast-forward
	MergeStyleRebase MergeStyle = "rebase"
	// MergeStyleRebaseMerge rebase before merging with merge commit (--no-ff)
	MergeStyleRebaseMerge MergeStyle = "rebase-merge"
	// MergeStyleSquash squash commits into single commit before merging
	MergeStyleSquash MergeStyle = "squash"
	// MergeStyleFastForwardOnly fast-forward merge if possible, otherwise fail
	MergeStyleFastForwardOnly MergeStyle = "fast-forward-only"
	// MergeStyleManuallyMerged pr has been merged manually, just mark it as merged directly
	MergeStyleManuallyMerged MergeStyle = "manually-merged"
	// MergeStyleRebaseUpdate not a merge style, used to update pull head by rebase
	MergeStyleRebaseUpdate MergeStyle = "rebase-update-only"
)

// UpdateStyle is a pull request branch update style
type UpdateStyle string

const (
	// UpdateStyleMerge merges the base branch into the pull request branch
	UpdateStyleMerge UpdateStyle = "merge"
	// UpdateStyleRebase rebases the pull request branch onto the base branch
	UpdateStyleRebase UpdateStyle = "rebase"
)

// SquashMergeMessage is the source of the default squash merge commit message body, values match GitHub's squash_merge_commit_message: https://docs.github.com/en/rest/repos/repos#update-a-repository
type SquashMergeMessage string

const (
	SquashMergeMessageBlank          SquashMergeMessage = "BLANK"
	SquashMergeMessageCommitMessages SquashMergeMessage = "COMMIT_MESSAGES"
	SquashMergeMessagePRBody         SquashMergeMessage = "PR_BODY"
)

func (m SquashMergeMessage) IsValid() bool {
	return m == SquashMergeMessageBlank || m == SquashMergeMessageCommitMessages || m == SquashMergeMessagePRBody
}

// UpdateDefaultBranch updates the default branch
func UpdateDefaultBranch(ctx context.Context, repo *Repository) error {
	_, err := db.GetEngine(ctx).ID(repo.ID).Cols("default_branch").Update(repo)
	return err
}

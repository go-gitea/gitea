// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/git"
	"gitea.dev/modules/util"
)

const (
	maxConflictDetailFiles   = 100
	maxConflictContentLength = 1024 * 1024
	maxConflictContentTotal  = 10 * 1024 * 1024
)

// ErrConflictRefNotFound is returned if the base branch or the head of the pull request doesn't exist any more
var ErrConflictRefNotFound = errors.New("base branch or head of the pull request not found")

// ErrConflictDetailUnsupported is returned if the installed git can't report merge conflict details
var ErrConflictDetailUnsupported = errors.New("conflict details need git 2.40 or newer")

// ConflictedFile is a conflicted file of a pull request
type ConflictedFile struct {
	*git.MergeTreeConflict
	// Content is the file of the merge result, it contains conflict markers when both sides changed it. It is nil if it is binary or too large.
	Content *string
}

// GetPullRequestConflicts merges the head of a pull request into its base branch without committing anything, and reports the conflicts
func GetPullRequestConflicts(ctx context.Context, gitRepo *git.Repository, pr *issues_model.PullRequest) (hasConflicts bool, conflicts []*ConflictedFile, truncated bool, _ error) {
	if !git.DefaultFeatures().SupportGitMergeTree {
		return false, nil, false, ErrConflictDetailUnsupported
	}
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return false, nil, false, err
	}

	baseCommitID, err := gitRepo.GetRefCommitID(ctx, git.BranchPrefix+pr.BaseBranch)
	if err != nil {
		return false, nil, false, fmt.Errorf("GetPullRequestConflicts: base branch %q: %w", pr.BaseBranch, util.Iif(errors.Is(err, util.ErrNotExist), ErrConflictRefNotFound, err))
	}
	headCommitID, err := gitRepo.GetRefCommitID(ctx, pr.GetGitHeadRefName())
	if err != nil {
		return false, nil, false, fmt.Errorf("GetPullRequestConflicts: head ref: %w", util.Iif(errors.Is(err, util.ErrNotExist), ErrConflictRefNotFound, err))
	}
	mergeBase, err := git.MergeBase(ctx, pr.BaseRepo, baseCommitID, headCommitID)
	if err != nil {
		return false, nil, false, err // util.ErrNotExist for unrelated histories
	}

	treeID, hasConflicts, mergeConflicts, truncated, err := git.MergeTreeConflicts(ctx, pr.BaseRepo, baseCommitID, headCommitID, mergeBase, maxConflictDetailFiles)
	if err != nil || len(mergeConflicts) == 0 {
		return hasConflicts, nil, false, err
	}

	tree, err := gitRepo.GetTree(ctx, treeID)
	if err != nil {
		return false, nil, false, fmt.Errorf("GetPullRequestConflicts: GetTree: %w", err)
	}
	totalContent := 0
	for _, mc := range mergeConflicts {
		conflict := &ConflictedFile{MergeTreeConflict: mc}
		blob, err := tree.GetBlobByPath(ctx, gitRepo, mc.Path)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return false, nil, false, fmt.Errorf("GetPullRequestConflicts: GetBlobByPath: %w", err)
		}
		if err == nil && blob.Size(ctx) <= maxConflictContentLength && totalContent+int(blob.Size(ctx)) <= maxConflictContentTotal {
			if content, err := blob.GetBlobContent(ctx, maxConflictContentLength); err != nil {
				return false, nil, false, fmt.Errorf("GetPullRequestConflicts: GetBlobContent: %w", err)
			} else if utf8.ValidString(content) && !strings.Contains(content, "\x00") {
				conflict.Content = &content
				totalContent += len(content)
			}
		}
		conflicts = append(conflicts, conflict)
	}
	return hasConflicts, conflicts, truncated, nil
}

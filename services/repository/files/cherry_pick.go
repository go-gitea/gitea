// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/pull"
)

// ErrCommitIDDoesNotMatch represents a "CommitIDDoesNotMatch" kind of error.
type ErrCommitIDDoesNotMatch struct {
	GivenCommitID   string
	CurrentCommitID string
}

// IsErrCommitIDDoesNotMatch checks if an error is a ErrCommitIDDoesNotMatch.
func IsErrCommitIDDoesNotMatch(err error) bool {
	_, ok := err.(ErrCommitIDDoesNotMatch)
	return ok
}

func (err ErrCommitIDDoesNotMatch) Error() string {
	return fmt.Sprintf("file CommitID does not match [given: %s, expected: %s]", err.GivenCommitID, err.CurrentCommitID)
}

// CherryPick cherry-picks or reverts a commit to the given repository
func CherryPick(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, revert bool, opts *ApplyDiffPatchOptions) (*structs.FileResponse, error) {
	if err := opts.Validate(ctx, repo, doer); err != nil {
		return nil, err
	}
	message := strings.TrimSpace(opts.Message)

	t, err := NewTemporaryUploadRepository(repo)
	if err != nil {
		log.Error("NewTemporaryUploadRepository failed: %v", err)
	}
	defer t.Close()
	if err := t.Clone(ctx, opts.OldBranch, false); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(ctx); err != nil {
		return nil, err
	}
	if err := t.RefreshIndex(ctx); err != nil {
		return nil, err
	}

	// Get the commit of the original branch
	commit, err := t.GetBranchCommit(opts.OldBranch)
	if err != nil {
		return nil, err // Couldn't get a commit for the branch
	}

	// Assigned LastCommitID in opts if it hasn't been set
	if opts.LastCommitID == "" {
		opts.LastCommitID = commit.ID.String()
	} else {
		lastCommitID, err := t.gitRepo.ConvertToGitID(opts.LastCommitID)
		if err != nil {
			return nil, fmt.Errorf("CherryPick: Invalid last commit ID: %w", err)
		}
		opts.LastCommitID = lastCommitID.String()
		if commit.ID.String() != opts.LastCommitID {
			return nil, ErrCommitIDDoesNotMatch{
				GivenCommitID:   opts.LastCommitID,
				CurrentCommitID: opts.LastCommitID,
			}
		}
	}

	commit, err = t.GetCommit(strings.TrimSpace(opts.Content))
	if err != nil {
		return nil, err
	}
	parent, err := commit.ParentID(0)
	if err != nil {
		parent = git.ObjectFormatFromName(repo.ObjectFormatName).EmptyTree()
	}

	base, right := parent.String(), commit.ID.String()

	if revert {
		right, base = base, right
	}

	description := fmt.Sprintf("CherryPick %s onto %s", right, opts.OldBranch)
	conflict, _, err := pull.AttemptThreeWayMerge(ctx,
		t.basePath, t.gitRepo, base, opts.LastCommitID, right, description)
	if err != nil {
		return nil, fmt.Errorf("failed to three-way merge %s onto %s: %w", right, opts.OldBranch, err)
	}

	if conflict {
		return nil, fmt.Errorf("failed to merge due to conflicts")
	}

	treeHash, err := t.WriteTree(ctx)
	if err != nil {
		// likely non-sensical tree due to merge conflicts...
		return nil, err
	}

	// Now commit the tree
	commitOpts := &CommitTreeUserOptions{
		ParentCommitID:    "HEAD",
		TreeHash:          treeHash,
		CommitMessage:     message,
		SignOff:           opts.Signoff,
		DoerUser:          doer,
		AuthorIdentity:    opts.Author,
		AuthorTime:        nil,
		CommitterIdentity: opts.Committer,
		CommitterTime:     nil,
	}
	if opts.Dates != nil {
		commitOpts.AuthorTime, commitOpts.CommitterTime = &opts.Dates.Author, &opts.Dates.Committer
	}
	commitHash, err := t.CommitTree(ctx, commitOpts)
	if err != nil {
		return nil, err
	}

	// Then push this tree to NewBranch
	if err := t.Push(ctx, doer, commitHash, opts.NewBranch); err != nil {
		return nil, err
	}

	commit, err = t.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	fileCommitResponse, _ := GetFileCommitResponse(repo, commit) // ok if fails, then will be nil
	verification := GetPayloadCommitVerification(ctx, commit)
	fileResponse := &structs.FileResponse{
		Commit:       fileCommitResponse,
		Verification: verification,
	}

	return fileResponse, nil
}

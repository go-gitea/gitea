// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package files

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
)

// CherryPick cherrypicks or reverts a commit to the given repository
func CherryPick(repo *repo_model.Repository, doer *user_model.User, revert bool, opts *ApplyDiffPatchOptions) (*structs.FileResponse, error) {
	if err := opts.Validate(repo, doer); err != nil {
		return nil, err
	}
	message := strings.TrimSpace(opts.Message)

	author, committer := GetAuthorAndCommitterUsers(opts.Author, opts.Committer, doer)

	t, err := NewTemporaryUploadRepository(repo)
	if err != nil {
		log.Error("%v", err)
	}
	defer t.Close()
	if err := t.Clone(opts.OldBranch); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(); err != nil {
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
		lastCommitID, err := t.gitRepo.ConvertToSHA1(opts.LastCommitID)
		if err != nil {
			return nil, fmt.Errorf("CherryPick: Invalid last commit ID: %v", err)
		}
		opts.LastCommitID = lastCommitID.String()
		if commit.ID.String() != opts.LastCommitID {
			return nil, models.ErrCommitIDDoesNotMatch{
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
		parent = git.MustIDFromString(git.EmptyTreeSHA)
	}

	base, right := parent.String(), commit.ID.String()

	if revert {
		right, base = base, right
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	err = git.NewCommand("read-tree", "-m", base, opts.LastCommitID, right).RunInDirFullPipeline(t.basePath, stdout, stderr, nil)
	if err != nil {
		return nil, fmt.Errorf("Error: Stdout: %s\nStderr: %s\nErr: %v", stdout.String(), stderr.String(), err)
	}

	treeHash, err := t.WriteTree()
	if err != nil {
		// likely non-sensical tree due to merge conflicts...
		return nil, err
	}

	// Now commit the tree
	var commitHash string
	if opts.Dates != nil {
		commitHash, err = t.CommitTreeWithDate(author, committer, treeHash, message, opts.Signoff, opts.Dates.Author, opts.Dates.Committer)
	} else {
		commitHash, err = t.CommitTree(author, committer, treeHash, message, opts.Signoff)
	}
	if err != nil {
		return nil, err
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		return nil, err
	}

	commit, err = t.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	fileCommitResponse, _ := GetFileCommitResponse(repo, commit) // ok if fails, then will be nil
	verification := GetPayloadCommitVerification(commit)
	fileResponse := &structs.FileResponse{
		Commit:       fileCommitResponse,
		Verification: verification,
	}

	return fileResponse, nil
}

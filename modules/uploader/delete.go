// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package uploader

import (
	"fmt"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
)

// DeleteRepoFileOptions holds the repository delete file options
type DeleteRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
}

// DeleteRepoFile deletes a file in the given repository
func DeleteRepoFile(repo *models.Repository, doer *models.User, opts *DeleteRepoFileOptions) error {
	t, err := NewTemporaryUploadRepository(repo)
	defer t.Close()
	if err != nil {
		return err
	}
	if err := t.Clone(opts.OldBranch); err != nil {
		return err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return err
	}

	filesInIndex, err := t.LsFiles(opts.TreePath)
	if err != nil {
		return fmt.Errorf("UpdateRepoFile: %v", err)
	}

	inFilelist := false
	for _, file := range filesInIndex {
		if file == opts.TreePath {
			inFilelist = true
		}
	}
	if !inFilelist {
		return git.ErrNotExist{RelPath: opts.TreePath}
	}

	if err := t.RemoveFilesFromIndex(opts.TreePath); err != nil {
		return err
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return err
	}

	// Now commit the tree
	commitHash, err := t.CommitTree(doer, treeHash, opts.Message)
	if err != nil {
		return err
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		return err
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}
	err = models.PushUpdate(
		opts.NewBranch,
		models.PushUpdateOptions{
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.Owner.Name,
			RepoName:     repo.Name,
			RefFullName:  git.BranchPrefix + opts.NewBranch,
			OldCommitID:  oldCommitID,
			NewCommitID:  commitHash,
		},
	)
	if err != nil {
		return fmt.Errorf("PushUpdate: %v", err)
	}

	// FIXME: Should we UpdateRepoIndexer(repo) here?
	return nil
}

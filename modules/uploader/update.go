// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package uploader

import (
	"fmt"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
)

// UpdateRepoFileOptions holds the repository file update options
type UpdateRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	OldTreeName  string
	NewTreeName  string
	Message      string
	Content      string
	IsNewFile    bool
}

// UpdateRepoFile adds or updates a file in the given repository
func UpdateRepoFile(repo *models.Repository, doer *models.User, opts UpdateRepoFileOptions) error {
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

	filesInIndex, err := t.LsFiles(opts.NewTreeName, opts.OldTreeName)
	if err != nil {
		return fmt.Errorf("UpdateRepoFile: %v", err)
	}

	if opts.IsNewFile {
		for _, file := range filesInIndex {
			if file == opts.NewTreeName {
				return models.ErrRepoFileAlreadyExist{FileName: opts.NewTreeName}
			}
		}
	}

	//var stdout string
	if opts.OldTreeName != opts.NewTreeName && len(filesInIndex) > 0 {
		for _, file := range filesInIndex {
			if file == opts.OldTreeName {
				if err := t.RemoveFilesFromIndex(opts.OldTreeName); err != nil {
					return err
				}
			}
		}

	}

	// Add the object to the database
	objectHash, err := t.HashObject(strings.NewReader(opts.Content))
	if err != nil {
		return err
	}

	// Add the object to the index
	if err := t.AddObjectToIndex("100644", objectHash, opts.NewTreeName); err != nil {
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
	models.UpdateRepoIndexer(repo)

	return nil
}

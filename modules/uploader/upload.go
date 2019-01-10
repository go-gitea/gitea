// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package uploader

import (
	"fmt"
	"os"
	"path"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
)

// UploadRepoFileOptions contains the uploaded repository file options
type UploadRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	Files        []string // In UUID format.
}

// UploadRepoFiles uploads files to the given repository
func UploadRepoFiles(repo *models.Repository, doer *models.User, opts UploadRepoFileOptions) error {
	if len(opts.Files) == 0 {
		return nil
	}

	uploads, err := models.GetUploadsByUUIDs(opts.Files)
	if err != nil {
		return fmt.Errorf("GetUploadsByUUIDs [uuids: %v]: %v", opts.Files, err)
	}

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

	// Copy uploaded files into repository.
	for _, upload := range uploads {
		file, err := os.Open(upload.LocalPath())
		if err != nil {
			return err
		}
		defer file.Close()

		objectHash, err := t.HashObject(file)
		if err != nil {
			return err
		}

		// Add the object to the index
		if err := t.AddObjectToIndex("100644", objectHash, path.Join(opts.TreePath, upload.Name)); err != nil {
			return err
		}
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
	// FIXME: Should we models.UpdateRepoIndexer(repo) here?

	return models.DeleteUploads(uploads...)
}

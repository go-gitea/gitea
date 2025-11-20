// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"errors"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/structs"
)

type DeleteRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	Signoff      bool
	Author       *IdentityOptions
	Committer    *IdentityOptions
}

// DeleteRepoFile deletes a file or directory in the given repository
func DeleteRepoFile(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, opts *DeleteRepoFileOptions) (*structs.FilesResponse, error) {
	if opts.TreePath == "" {
		return nil, errors.New("path cannot be empty")
	}

	// If no branch name is set, assume the default branch
	if opts.OldBranch == "" {
		opts.OldBranch = repo.DefaultBranch
	}
	if opts.NewBranch == "" {
		opts.NewBranch = opts.OldBranch
	}

	gitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// Get commit
	commit, err := gitRepo.GetBranchCommit(opts.OldBranch)
	if err != nil {
		return nil, err
	}

	// Get entry
	entry, err := commit.GetTreeEntryByPath(opts.TreePath)
	if err != nil {
		return nil, err
	}

	var filesToDelete []*ChangeRepoFile

	if entry.IsDir() {
		tree, err := commit.SubTree(opts.TreePath)
		if err != nil {
			return nil, err
		}

		entries, err := tree.ListEntriesRecursiveFast()
		if err != nil {
			return nil, err
		}

		for _, e := range entries {
			if !e.IsDir() && !e.IsSubModule() {
				filesToDelete = append(filesToDelete, &ChangeRepoFile{
					Operation: "delete",
					TreePath:  opts.TreePath + "/" + e.Name(),
				})
			}
		}
	} else {
		filesToDelete = append(filesToDelete, &ChangeRepoFile{
			Operation: "delete",
			TreePath:  opts.TreePath,
		})
	}

	return ChangeRepoFiles(ctx, repo, doer, &ChangeRepoFilesOptions{
		LastCommitID: opts.LastCommitID,
		OldBranch:    opts.OldBranch,
		NewBranch:    opts.NewBranch,
		Message:      opts.Message,
		Files:        filesToDelete,
		Signoff:      opts.Signoff,
		Author:       opts.Author,
		Committer:    opts.Committer,
	})
}

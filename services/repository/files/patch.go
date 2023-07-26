// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
)

// ApplyDiffPatchOptions holds the repository diff patch update options
type ApplyDiffPatchOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	Message      string
	Content      string
	SHA          string
	Author       *IdentityOptions
	Committer    *IdentityOptions
	Dates        *CommitDateOptions
	Signoff      bool
}

// Validate validates the provided options
func (opts *ApplyDiffPatchOptions) Validate(ctx context.Context, repo *repo_model.Repository, doer *user_model.User) error {
	// If no branch name is set, assume master
	if opts.OldBranch == "" {
		opts.OldBranch = repo.DefaultBranch
	}
	if opts.NewBranch == "" {
		opts.NewBranch = opts.OldBranch
	}

	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return err
	}
	defer closer.Close()

	// oldBranch must exist for this operation
	if _, err := gitRepo.GetBranch(opts.OldBranch); err != nil {
		return err
	}
	// A NewBranch can be specified for the patch to be applied to.
	// Check to make sure the branch does not already exist, otherwise we can't proceed.
	// If we aren't branching to a new branch, make sure user can commit to the given branch
	if opts.NewBranch != opts.OldBranch {
		existingBranch, err := gitRepo.GetBranch(opts.NewBranch)
		if existingBranch != nil {
			return git_model.ErrBranchAlreadyExists{
				BranchName: opts.NewBranch,
			}
		}
		if err != nil && !git.IsErrBranchNotExist(err) {
			return err
		}
	} else {
		protectedBranch, err := git_model.GetFirstMatchProtectedBranchRule(ctx, repo.ID, opts.OldBranch)
		if err != nil {
			return err
		}
		if protectedBranch != nil {
			protectedBranch.Repo = repo
			if !protectedBranch.CanUserPush(ctx, doer) {
				return models.ErrUserCannotCommit{
					UserName: doer.LowerName,
				}
			}
		}
		if protectedBranch != nil && protectedBranch.RequireSignedCommits {
			_, _, _, err := asymkey_service.SignCRUDAction(ctx, repo.RepoPath(), doer, repo.RepoPath(), opts.OldBranch)
			if err != nil {
				if !asymkey_service.IsErrWontSign(err) {
					return err
				}
				return models.ErrUserCannotCommit{
					UserName: doer.LowerName,
				}
			}
		}
	}
	return nil
}

// ApplyDiffPatch applies a patch to the given repository
func ApplyDiffPatch(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, opts *ApplyDiffPatchOptions) (*structs.FileResponse, error) {
	if err := opts.Validate(ctx, repo, doer); err != nil {
		return nil, err
	}

	message := strings.TrimSpace(opts.Message)

	author, committer := GetAuthorAndCommitterUsers(opts.Author, opts.Committer, doer)

	t, err := NewTemporaryUploadRepository(ctx, repo)
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
			return nil, fmt.Errorf("ApplyPatch: Invalid last commit ID: %w", err)
		}
		opts.LastCommitID = lastCommitID.String()
		if commit.ID.String() != opts.LastCommitID {
			return nil, models.ErrCommitIDDoesNotMatch{
				GivenCommitID:   opts.LastCommitID,
				CurrentCommitID: opts.LastCommitID,
			}
		}
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	cmdApply := git.NewCommand(ctx, "apply", "--index", "--recount", "--cached", "--ignore-whitespace", "--whitespace=fix", "--binary")
	if git.CheckGitVersionAtLeast("2.32") == nil {
		cmdApply.AddArguments("-3")
	}

	if err := cmdApply.Run(&git.RunOpts{
		Dir:    t.basePath,
		Stdout: stdout,
		Stderr: stderr,
		Stdin:  strings.NewReader(opts.Content),
	}); err != nil {
		return nil, fmt.Errorf("Error: Stdout: %s\nStderr: %s\nErr: %w", stdout.String(), stderr.String(), err)
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return nil, err
	}

	// Now commit the tree
	var commitHash string
	if opts.Dates != nil {
		commitHash, err = t.CommitTreeWithDate("HEAD", author, committer, treeHash, message, opts.Signoff, opts.Dates.Author, opts.Dates.Committer)
	} else {
		commitHash, err = t.CommitTree("HEAD", author, committer, treeHash, message, opts.Signoff)
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
	verification := GetPayloadCommitVerification(ctx, commit)
	fileResponse := &structs.FileResponse{
		Commit:       fileCommitResponse,
		Verification: verification,
	}

	return fileResponse, nil
}

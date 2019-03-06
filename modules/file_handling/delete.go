// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package file_handling

import (
	"fmt"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/sdk/gitea"
)

// DeleteRepoFileOptions holds the repository delete file options
type DeleteRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	SHA          string
	Author       *IdentityOptions
	Committer    *IdentityOptions
}

// DeleteRepoFile deletes a file in the given repository
func DeleteRepoFile(repo *models.Repository, doer *models.User, opts *DeleteRepoFileOptions) (*gitea.FileResponse, error) {
	if repo == nil {
		return nil, fmt.Errorf("repo not passed to DeleteRepoFile")
	}
	if doer == nil {
		return nil, fmt.Errorf("doer not passed to DeleteRepoFile")
	}
	if opts == nil {
		return nil, fmt.Errorf("opts not passed to DeleteRepoFile")
	}

	// If no branch name is set, assume master
	if opts.OldBranch == "" {
		opts.OldBranch = "master"
	}
	if opts.NewBranch == "" {
		opts.NewBranch = opts.OldBranch
	}

	// oldBranch must exist for this operation
	if _, err := repo.GetBranch(opts.OldBranch); err != nil {
		return nil, err
	}

	// A NewBranch can be specified for the file to be created/updated in a new branch
	// Check to make sure the branch does not already exist, otherwise we can't proceed.
	// If we aren't branching to a new branch, make sure user can commit to the given branch
	if opts.NewBranch != opts.OldBranch {
		newBranch, err := repo.GetBranch(opts.NewBranch)
		if git.IsErrNotExist(err) {
			return nil, err
		}
		if newBranch != nil {
			return nil, models.ErrBranchAlreadyExists{opts.NewBranch}
		}
	} else {
		if protected, _ := repo.IsProtectedBranchForPush(opts.OldBranch, doer); protected {
			return nil, models.ErrCannotCommit{UserName: doer.LowerName}
		}
	}

	// Check that the path given in opts.treeName is valid (not a git path)
	treePath := cleanUploadFileName(opts.TreePath)
	if treePath == "" {
		return nil, models.ErrFilenameInvalid{opts.TreePath}
	}

	message := strings.TrimSpace(opts.Message)

	// Committer and author are optional. If they are not the doer (not same email address)
	// then we use bogus User objects for them to store their FullName and Email.
	// If only one of the two are provided, we set both of them to it.
	// If neither are provided, both are the doer.
	var committer *models.User
	var author *models.User
	if opts.Committer != nil && opts.Committer.Email == "" {
		if strings.ToLower(doer.Email) == strings.ToLower(opts.Committer.Email) {
			committer = doer // the committer is the doer, so will use their user object
		} else {
			committer = &models.User{
				FullName: opts.Committer.Name,
				Email: opts.Committer.Email,
			}
		}
	}
	if opts.Author != nil && opts.Author.Email == "" {
		if strings.ToLower(doer.Email) == strings.ToLower(opts.Author.Email) {
			author = doer // the author is the doer, so will use their user object
		} else {
			author = &models.User{
				FullName: opts.Author.Name,
				Email: opts.Author.Email,
			}
		}
	}
	if author == nil {
		if committer != nil {
			author = committer // No valid author was given so use the committer
		} else {
			author = doer // No valid author was given and no valid committer so use the doer
		}
	}
	if committer == nil {
		committer = author // No valid committer so use the author as the committer (was set to a valid user above)
	}

	t, err := NewTemporaryUploadRepository(repo)
	defer t.Close()
	if err != nil {
		return nil, err
	}
	if err := t.Clone(opts.OldBranch); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return nil, err
	}

	if opts.LastCommitID == "" {
		if commitID, err := t.GetLastCommit(); err != nil {
			return nil, err
		} else {
			opts.LastCommitID = commitID
		}
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}

	// Get the commit of the original branch
	commit, err := gitRepo.GetBranchCommit(opts.OldBranch)
	if err != nil {
		return nil, err // Couldn't get a commit for the branch
	}

	filesInIndex, err := t.LsFiles(opts.TreePath)
	if err != nil {
		return nil, fmt.Errorf("DeleteRepoFile: %v", err)
	}

	inFilelist := false
	for _, file := range filesInIndex {
		if file == opts.TreePath {
			inFilelist = true
		}
	}
	if !inFilelist {
		return nil, git.ErrNotExist{RelPath: opts.TreePath}
	}

	// Get the entry of treePath and check if the SHA given is the same as the file
	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}
	if opts.SHA != "" && opts.SHA != entry.ID.String() {
		return nil, models.ErrShaDoesNotMatch{
			GivenSHA:   opts.SHA,
			CurrentSHA: entry.ID.String(),
		}
	}

	if err := t.RemoveFilesFromIndex(opts.TreePath); err != nil {
		return nil, err
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return nil, err
	}

	// Now commit the tree
	commitHash, err := t.CommitTree(author, committer, treeHash, message)
	if err != nil {
		return nil, err
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		return nil, err
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return nil, fmt.Errorf("GetOwner: %v", err)
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
		return nil, fmt.Errorf("PushUpdate: %v", err)
	}

	// FIXME: Should we UpdateRepoIndexer(repo) here?

	if file, err := GetFileResponseFromCommit(repo, commit, opts.NewBranch, treePath); err != nil {
		return nil, err
	} else {
		return file, nil
	}
}

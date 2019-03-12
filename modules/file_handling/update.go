// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package file_handling

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/sdk/gitea"
	"fmt"
	"path"
	"strings"
)

// IdentityOptions for a person's identity like an author or committer
type IdentityOptions struct {
	Name  string
	Email string
}

// UpdateRepoFileOptions holds the repository file update options
type UpdateRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	FromTreePath string
	Message      string
	Content      string
	SHA          string
	IsNewFile    bool
	Author       *IdentityOptions
	Committer    *IdentityOptions
}

// CreateOrUpdateRepoFile adds or updates a file in the given repository
func CreateOrUpdateRepoFile(repo *models.Repository, doer *models.User, opts *UpdateRepoFileOptions) (*gitea.FileResponse, error) {
	if repo == nil {
		return nil, fmt.Errorf("repo cannot be nil")
	}
	if doer == nil {
		return nil, fmt.Errorf("doer cannot be nil")
	}
	if opts == nil {
		return nil, fmt.Errorf("opts cannot be nil")
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

	// If FromTreePath is not set, set it to the opts.TreePath
	if opts.TreePath != "" && opts.FromTreePath == "" {
		opts.FromTreePath = opts.TreePath
	}

	// Check that the path given in opts.treeName is valid (not a git path)
	treeName := CleanUploadFileName(opts.TreePath)
	if treeName == "" {
		return nil, models.ErrFilenameInvalid{opts.TreePath}
	}
	// If there is a fromTreeName (we are copying it), also clean it up
	fromTreeName := CleanUploadFileName(opts.FromTreePath)
	if fromTreeName == "" && opts.FromTreePath != "" {
		return nil, models.ErrFilenameInvalid{opts.FromTreePath}
	}

	message := strings.TrimSpace(opts.Message)

	author, committer := GetAuthorAndCommitterUsers(opts.Committer, opts.Author, doer)

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

	// Get the entry of fromTreeName and check if the SHA given, if updating, is the same
	if opts.SHA != "" {
		fromEntry, err := commit.GetTreeEntryByPath(fromTreeName)
		if err != nil {
			return nil, err
		}
		if opts.SHA != fromEntry.ID.String() {
			return nil, models.ErrShaDoesNotMatch{
				GivenSHA:   opts.SHA,
				CurrentSHA: fromEntry.ID.String(),
			}
		}
	}

	// For the path where this file will be created/updated, we need to make
	// sure no parts of the path are existing files or links except for the last
	// item in the path which is the file name, and that shouldn't exist IF it is
	// a new file OR is being moved to a new path.
	treeNameParts := strings.Split(treeName, "/")
	subTreeName := ""
	for index, part := range treeNameParts {
		subTreeName = path.Join(subTreeName, part)
		entry, err := commit.GetTreeEntryByPath(subTreeName)
		if err != nil {
			if git.IsErrNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}
			return nil, err
		}
		if index < len(treeNameParts)-1 {
			if !entry.IsDir() {
				return nil, models.ErrWithFilePath{fmt.Sprintf("%s is not a directory, it is a file", subTreeName)}
			}
		} else if entry.IsLink() {
			return nil, models.ErrWithFilePath{fmt.Sprintf("%s is not a file, it is a symbolic link", subTreeName)}
		} else if entry.IsDir() {
			return nil, models.ErrWithFilePath{fmt.Sprintf("%s is not a file, it is a directory", subTreeName)}
		} else if fromTreeName != treeName || opts.IsNewFile {
			// The entry shouldn't exist if we are creating new file or moving to a new path
			return nil, models.ErrRepoFileAlreadyExists{FileName: treeName}
		}

	}

	// Get the two paths (might be the same if not moving) from the index if they exist
	filesInIndex, err := t.LsFiles(opts.TreePath, opts.FromTreePath)
	if err != nil {
		return nil, fmt.Errorf("UpdateRepoFile: %v", err)
	}
	// If is a new file (not updating) then the given path shouldn't exist
	if opts.IsNewFile {
		for _, file := range filesInIndex {
			if file == opts.TreePath {
				return nil, models.ErrRepoFileAlreadyExists{FileName: opts.TreePath}
			}
		}
	}

	// Remove the old path from the tree
	if fromTreeName != treeName && len(filesInIndex) > 0 {
		for _, file := range filesInIndex {
			if file == fromTreeName {
				if err := t.RemoveFilesFromIndex(opts.FromTreePath); err != nil {
					return nil, err
				}
			}
		}
	}

	// Check there is no way this can return multiple infos
	filename2attribute2info, err := t.CheckAttribute("filter", treeName)
	if err != nil {
		return nil, err
	}

	content := opts.Content
	var lfsMetaObject *models.LFSMetaObject

	if filename2attribute2info[treeName] != nil && filename2attribute2info[treeName]["filter"] == "lfs" {
		// OK so we are supposed to LFS this data!
		oid, err := models.GenerateLFSOid(strings.NewReader(opts.Content))
		if err != nil {
			return nil, err
		}
		lfsMetaObject = &models.LFSMetaObject{Oid: oid, Size: int64(len(opts.Content)), RepositoryID: repo.ID}
		content = lfsMetaObject.Pointer()
	}

	// Add the object to the database
	objectHash, err := t.HashObject(strings.NewReader(content))
	if err != nil {
		return nil, err
	}

	// Add the object to the index
	if err := t.AddObjectToIndex("100644", objectHash, treeName); err != nil {
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

	if lfsMetaObject != nil {
		// We have an LFS object - create it
		lfsMetaObject, err = models.NewLFSMetaObject(lfsMetaObject)
		if err != nil {
			return nil, err
		}
		contentStore := &lfs.ContentStore{BasePath: setting.LFS.ContentPath}
		if !contentStore.Exists(lfsMetaObject) {
			if err := contentStore.Put(lfsMetaObject, strings.NewReader(opts.Content)); err != nil {
				if err2 := repo.RemoveLFSMetaObjectByOid(lfsMetaObject.Oid); err2 != nil {
					return nil, fmt.Errorf("Error whilst removing failed inserted LFS object %s: %v (Prev Error: %v)", lfsMetaObject.Oid, err2, err)
				}
				return nil, err
			}
		}
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		return nil, err
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch || oldCommitID == "" {
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
	models.UpdateRepoIndexer(repo)

	commit, err = gitRepo.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	if file, err := GetFileResponseFromCommit(repo, commit, opts.NewBranch, treeName); err != nil {
		return nil, err
	} else {
		return file, nil
	}
}

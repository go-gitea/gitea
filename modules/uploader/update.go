// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package uploader

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
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
	TreeName     string
	FromTreeName string
	Message      string
	Content      string
	SHA          string
	IsNewFile    bool
	Author       *IdentityOptions
	Committer    *IdentityOptions
}

// CreateOrUpdateRepoFile adds or updates a file in the given repository
func CreateOrUpdateRepoFile(repo *models.Repository, doer *models.User, opts *UpdateRepoFileOptions) (*File, error) {
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

	// If FromTreeName is not set, set it to the opts.TreeName
	if opts.TreeName != "" && opts.FromTreeName == "" {
		opts.FromTreeName = opts.TreeName
	}

	log.Warn("%v", opts)

	// Check that the path given in opts.treeName is valid (not a git path)
	treeName := cleanUploadFileName(opts.TreeName)
	if treeName == "" {
		return nil, models.ErrFilenameInvalid{opts.TreeName}
	}
	// If there is a fromTreeName (we are copying it), also clean it up
	fromTreeName := cleanUploadFileName(opts.FromTreeName)
	if fromTreeName == "" && opts.FromTreeName != "" {
		return nil, models.ErrFilenameInvalid{opts.FromTreeName}
	}

	message := strings.TrimSpace(opts.Message)

	var committer *models.User
	var author *models.User
	if opts.Committer != nil && opts.Committer.Email == "" {
		if c, err := models.GetUserByEmail(opts.Committer.Email); err != nil {
			committer = doer
		} else {
			committer = c
		}
	}
	if opts.Author != nil && opts.Author.Email == "" {
		if a, err := models.GetUserByEmail(opts.Author.Email); err != nil {
			author = doer
		} else {
			author = a
		}
	}
	if author == nil {
		if committer != nil {
			author = committer
		} else {
			author = doer
		}
	}
	if committer == nil {
		committer = author
	}
	doer = committer // UNTIL WE FIGURE OUT HOW TO ADD AUTHOR AND COMMITTER, USING JUST COMMITTER

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

	// Check to see if we are needing to move this updated file to a new file name
	// If so, we make sure the new file name doesn't already exist (cannot clobber)
	if !opts.IsNewFile && treeName != fromTreeName {
		//if entry, err := commit.GetTreeEntryByPath(treeName); err != nil {
		//	// If it wasn't a ErrNotExist error, it was something else so return it
		//	if !git.IsErrNotExist(err) {
		//		return nil, err
		//	}
		//} else if entry != nil {
		//	// Otherwise, if no error and the entry exists, we can't make the file
		//	return nil, models.ErrRepoFileAlreadyExists{treeName}
		//}
	}

	// For the path where this file will be created/updated, we need to make
	// sure no parts of the path are existing files or links except for the last
	// item in the path which is the file name
	treeNameParts := strings.Split(treeName, "/")
	subTreeName := ""
	for index, part := range treeNameParts {
		subTreeName = path.Join(subTreeName, part)
		log.Warn("GETTING ENTRY FOR %s", subTreeName)
		entry, err := commit.GetTreeEntryByPath(treeName)
		log.Warn("ERROR? %v", err)
		if err != nil {
			if git.IsErrNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}
			return nil, err
		}
		log.Warn("HERE: %s %v", entry.ID, entry.IsDir())
		if index < len(treeNameParts)-1 {
			if !entry.IsDir() {
				return nil, models.ErrWithFilePath{fmt.Sprintf("%s is not a directory, it is a file", subTreeName)}
			}
		} else {
			if entry.IsLink() {
				return nil, models.ErrWithFilePath{fmt.Sprintf("%s is not a file, it is a symbolic link", subTreeName)}
			}
			if entry.IsDir() {
				return nil, models.ErrWithFilePath{fmt.Sprintf("%s is not a file, it is a directory", subTreeName)}
			}
		}
	}

	filesInIndex, err := t.LsFiles(opts.TreeName, opts.FromTreeName)
	if err != nil {
		return nil, fmt.Errorf("UpdateRepoFile: %v", err)
	}
	j, err := json.Marshal(filesInIndex)
	log.Warn("FILESININDEX: %v", j)
	for idx, file := range filesInIndex {
		log.Warn("FILE: %d: %s", idx, file)
	}

	if opts.IsNewFile {
		for _, file := range filesInIndex {
			log.Warn("FILE: %s", file)
			if file == opts.TreeName {
				return nil, models.ErrRepoFileAlreadyExists{FileName: opts.TreeName}
			}
		}
	}

	//var stdout string
	if fromTreeName != treeName && len(filesInIndex) > 0 {
		for _, file := range filesInIndex {
			if file == fromTreeName {
				if err := t.RemoveFilesFromIndex(opts.FromTreeName); err != nil {
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
	commitHash, err := t.CommitTree(doer, treeHash, message)
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

	return &File{}, nil
}

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

//import (
//	"code.gitea.io/git"
//	"code.gitea.io/gitea/modules/lfs"
//	"code.gitea.io/gitea/modules/setting"
//	"fmt"
//	"path"
//	"strings"
//)
//
//// FileLink contains the links for a repo's file
//type FileLink struct {
//	Self    string
//	GitURL  string
//	HTMLURL string
//}
//
//// FileContent contains information about a repo's file stats and content
//type FileContent struct {
//	Name        string
//	Path        string
//	SHA         string
//	Size        int64
//	URL         string
//	HTMLURL     string
//	GitURL      string
//	DownloadURL string
//	Type        string
//	Links       []*FileLink
//}
//
//type CommitMeta struct {
//	URL string
//	SHA string
//}
//
//// CommitUser contains information of a user in the context of a commit.
//type CommitUser struct {
//	Name  string
//	Email string
//	Date  string
//}
//
//// FileCommit contains information generated from a Git commit for a repo's file.
//type FileCommit struct {
//	CommitMeta
//	HTMLURL   string
//	Author    *CommitUser
//	Committer *CommitUser
//	Parents   []*CommitMeta
//	NodeID    string
//	Message   string
//	Tree      *CommitMeta
//}
//
//// PayloadCommitVerification represents the GPG verification of a commit
//type PayloadCommitVerification struct {
//	Verified  bool
//	Reason    string
//	Signature string
//	Payload   string
//}
//
//// File contains information about a repo's file
//type File struct {
//	Content      *FileContent
//	Commit       *FileCommit
//	Verification *PayloadCommitVerification
//}
//
//// IdentityOptions for a person's identity like an author or committer
//type IdentityOptions struct {
//	Name  string
//	Email string
//}
//
//// FileOptions contains options for files
//type FileOptions struct {
//	Message     string
//	Content     string
//	OldBranch   string
//	NewBranch   string
//	NewTreePath string
//	OldTreePath string
//	IsNewFile   bool
//	Commit      *git.Commit
//	Author      *IdentityOptions
//	Committer   *IdentityOptions
//}
//
//func cleanUploadFileName(name string) string {
//	// Rebase the filename
//	name = strings.Trim(path.Clean("/"+name), " /")
//	// Git disallows any filenames to have a .git directory in them.
//	for _, part := range strings.Split(name, "/") {
//		if strings.ToLower(part) == ".git" {
//			return ""
//		}
//	}
//	return name
//}
//
//func CreateOrUpdateFile(doer *User, repo *Repository, gitRepo *git.Repository, opts FileOptions) (*File, error) {
//	// If no branch name is set, assume master
//	if opts.OldBranch == "" {
//		opts.OldBranch = "master"
//	}
//
//	// "BranchName" must exist for this operation
//	if _, err := repo.GetBranch(opts.OldBranch); err != nil {
//		return nil, err
//	}
//
//	// A NewBranch can be specified for the file to be created/updated in a new branch
//	// Check to make sure the branch does not already exist, otherwise we can't proceed.
//	// If we aren't branching to a new branch, make sure user can commit to the given branch
//	if opts.NewBranch != "" {
//		newBranch, err := repo.GetBranch(opts.NewBranch)
//		if git.IsErrNotExist(err) {
//			return nil, err
//		}
//		if newBranch != nil {
//			return nil, ErrBranchAlreadyExists{opts.NewBranch}
//		}
//	} else {
//		if protected, _ := repo.IsProtectedBranchForPush(opts.OldBranch, user); protected {
//			return nil, ErrCannotCommit{UserName: user.LowerName}
//		}
//	}
//
//	// Check that the path given in opts.Path is valid (not a git path)
//	// and if an OldTreePath was given, to also check it
//	newTreePath := cleanUploadFileName(opts.NewTreePath)
//	if len(newTreePath) == 0 {
//		return nil, ErrFilenameInvalid{opts.NewTreePath}
//	}
//	origTreePath := ""
//	if opts.OldTreePath == "" {
//		opts.OldTreePath = newTreePath
//		origTreePath = cleanUploadFileName(opts.OldTreePath)
//		if len(opts.OldTreePath) > 0 && len(origTreePath) == 0 {
//			return nil, ErrFilenameInvalid{opts.OldTreePath}
//		}
//	} else {
//		origTreePath = newTreePath
//	}
//
//	// Get the commit of the original branch
//	commit, err := gitRepo.GetBranchCommit(opts.OldBranch)
//	if err != nil {
//		return nil, err // Couldn't get a commit for the branch
//	}
//
//	// Check to see if we are needing to move this updated file to a new file name
//	// If so, we make sure the new file name doesn't already exist (cannot clobber)
//	if !opts.IsNewFile && origTreePath != newTreePath {
//		if entry, err := commit.GetTreeEntryByPath(newTreePath); err != nil {
//			if !git.IsErrNotExist(err) {
//				return nil, err
//			}
//		} else if entry != nil {
//			return nil, ErrRepoFileAlreadyExist{newTreePath}
//		}
//	}
//
//	// For the path where this file will be created/updated, we need to make
//	// sure no parts of the path are existing files or links except for the last
//	// item in the path which is the file name
//	treePathParts := strings.Split(newTreePath, "/")
//	for index, part := range treePathParts {
//		newTreePath = path.Join(newTreePath, part)
//		entry, err := commit.GetTreeEntryByPath(newTreePath)
//		if err != nil {
//			if git.IsErrNotExist(err) {
//				// Means there is no item with that name, so we're good
//				break
//			}
//			return nil, err
//		}
//		if index < len(treePathParts)-1 {
//			if !entry.IsDir() {
//				return nil, ErrWithFilePath{fmt.Sprintf("%s is not a directory, it is a file", newTreePath)}
//			}
//		} else {
//			if entry.IsLink() {
//				return nil, ErrWithFilePath{fmt.Sprintf("%s is not a file, it is a symbolic link", newTreePath)}
//			}
//			if entry.IsDir() {
//				return nil, ErrWithFilePath{fmt.Sprintf("%s is not a file, it is a directory", newTreePath)}
//			}
//		}
//	}
//
//	message := strings.TrimSpace(opts.Message)
//
//	var committer *User
//	var author *User
//	if opts.Committer.Email == "" {
//		committer, err = GetUserByEmail(opts.Committer.Email)
//		if err != nil {
//			return nil, err
//		}
//	}
//	if opts.Author.Email == "" {
//		author, err = GetUserByEmail(opts.Author.Email)
//		if err != nil {
//			return nil, err
//		}
//	}
//	if author == nil {
//		if committer != nil {
//			author = committer
//		} else {
//			author = doer
//		}
//	}
//	if committer == nil {
//		committer = author
//	}
//	doer = committer
//
//
//
//	t, err := NewTemporaryUploadRepository(repo)
//	defer t.Close()
//	if err != nil {
//		return err
//	}
//	if err := t.Clone(opts.OldBranch); err != nil {
//		return err
//	}
//	if err := t.SetDefaultIndex(); err != nil {
//		return err
//	}
//
//	filesInIndex, err := t.LsFiles(opts.NewTreeName, opts.OldTreeName)
//	if err != nil {
//		return fmt.Errorf("UpdateRepoFile: %v", err)
//	}
//
//	if opts.IsNewFile {
//		for _, file := range filesInIndex {
//			if file == opts.NewTreeName {
//				return models.ErrRepoFileAlreadyExist{FileName: opts.NewTreeName}
//			}
//		}
//	}
//
//	//var stdout string
//	if opts.OldTreeName != opts.NewTreeName && len(filesInIndex) > 0 {
//		for _, file := range filesInIndex {
//			if file == opts.OldTreeName {
//				if err := t.RemoveFilesFromIndex(opts.OldTreeName); err != nil {
//					return err
//				}
//			}
//		}
//	}
//
//	// Check there is no way this can return multiple infos
//	filename2attribute2info, err := t.CheckAttribute("filter", opts.NewTreeName)
//	if err != nil {
//		return err
//	}
//
//	content := opts.Content
//	var lfsMetaObject *models.LFSMetaObject
//
//	if filename2attribute2info[opts.NewTreeName] != nil && filename2attribute2info[opts.NewTreeName]["filter"] == "lfs" {
//		// OK so we are supposed to LFS this data!
//		oid, err := models.GenerateLFSOid(strings.NewReader(opts.Content))
//		if err != nil {
//			return err
//		}
//		lfsMetaObject = &models.LFSMetaObject{Oid: oid, Size: int64(len(opts.Content)), RepositoryID: repo.ID}
//		content = lfsMetaObject.Pointer()
//	}
//
//	// Add the object to the database
//	objectHash, err := t.HashObject(strings.NewReader(content))
//	if err != nil {
//		return err
//	}
//
//	// Add the object to the index
//	if err := t.AddObjectToIndex("100644", objectHash, opts.NewTreeName); err != nil {
//		return err
//	}
//
//	// Now write the tree
//	treeHash, err := t.WriteTree()
//	if err != nil {
//		return err
//	}
//
//	// Now commit the tree
//	commitHash, err := t.CommitTree(doer, treeHash, opts.Message)
//	if err != nil {
//		return err
//	}
//
//	if lfsMetaObject != nil {
//		// We have an LFS object - create it
//		lfsMetaObject, err = models.NewLFSMetaObject(lfsMetaObject)
//		if err != nil {
//			return err
//		}
//		contentStore := &lfs.ContentStore{BasePath: setting.LFS.ContentPath}
//		if !contentStore.Exists(lfsMetaObject) {
//			if err := contentStore.Put(lfsMetaObject, strings.NewReader(opts.Content)); err != nil {
//				if err2 := repo.RemoveLFSMetaObjectByOid(lfsMetaObject.Oid); err2 != nil {
//					return fmt.Errorf("Error whilst removing failed inserted LFS object %s: %v (Prev Error: %v)", lfsMetaObject.Oid, err2, err)
//				}
//				return err
//			}
//		}
//	}
//
//	// Then push this tree to NewBranch
//	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
//		return err
//	}
//
//	// Simulate push event.
//	oldCommitID := opts.LastCommitID
//	if opts.NewBranch != opts.OldBranch {
//		oldCommitID = git.EmptySHA
//	}
//
//	if err = repo.GetOwner(); err != nil {
//		return fmt.Errorf("GetOwner: %v", err)
//	}
//	err = models.PushUpdate(
//		opts.NewBranch,
//		models.PushUpdateOptions{
//			PusherID:     doer.ID,
//			PusherName:   doer.Name,
//			RepoUserName: repo.Owner.Name,
//			RepoName:     repo.Name,
//			RefFullName:  git.BranchPrefix + opts.NewBranch,
//			OldCommitID:  oldCommitID,
//			NewCommitID:  commitHash,
//		},
//	)
//	if err != nil {
//		return fmt.Errorf("PushUpdate: %v", err)
//	}
//	models.UpdateRepoIndexer(repo)
//
//
//
//	file := &File{}
//
//	return file, nil
//}

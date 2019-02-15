// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/uploader"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"fmt"
	"path"
	"strings"
)

// FileLink contains the links for a repo's file
type FileLink struct {
	Self    string
	GitURL  string
	HTMLURL string
}

// FileContent contains information about a repo's file stats and content
type FileContent struct {
	Name        string
	Path        string
	SHA         string
	Size        int64
	URL         string
	HTMLURL     string
	GitURL      string
	DownloadURL string
	Type        string
	Links       []*FileLink
}

type CommitMeta struct {
	URL string
	SHA string
}

// CommitUser contains information of a user in the context of a commit.
type CommitUser struct {
	Name  string
	Email string
	Date  string
}

// FileCommit contains information generated from a Git commit for a repo's file.
type FileCommit struct {
	CommitMeta
	HTMLURL   string
	Author    *CommitUser
	Committer *CommitUser
	Parents   []*CommitMeta
	NodeID    string
	Message   string
	Tree      *CommitMeta
}

// PayloadCommitVerification represents the GPG verification of a commit
type PayloadCommitVerification struct {
	Verified  bool
	Reason    string
	Signature string
	Payload   string
}

// File contains information about a repo's file
type File struct {
	Content      *FileContent
	Commit       *FileCommit
	Verification *PayloadCommitVerification
}

// IdentityOptions for a person's identity like an author or committer
type IdentityOptions struct {
	Name string
	Email string
}

// FileOptions contains options for files
type FileOptions struct {
	Message   string
	Content   string
	BranchName    string
	NewBranchName string
	Path      string
	FromPath   string
	SHA       string
	Author    IdentityOptions
	Committer IdentityOptions
}

func cleanUploadFileName(name string) string {
	// Rebase the filename
	name = strings.Trim(path.Clean("/"+name), " /")
	// Git disallows any filenames to have a .git directory in them.
	for _, part := range strings.Split(name, "/") {
		if strings.ToLower(part) == ".git" {
			return ""
		}
	}
	return name
}

func CreateOrUpdateFile(user *User, repo *Repository, gitRepo *git.Repository, opts FileOptions) (*File, error) {
	// If no branch name is set, assume master
	if opts.BranchName == "" {
		opts.BranchName = "master"
	}

	// "BranchName" must exist for this operation
	if _, err := repo.GetBranch(opts.BranchName); err != nil {
		return nil, err
	}

	// A NewBranchName can be specified for the file to be created/updated in a new branch
	// Check to make sure the branch does not already exist, otherwise we can't proceed.
	// If we aren't branching to a new branch, make sure user can commit to the given branch
	if opts.NewBranchName != "" {
		newBranch, err := repo.GetBranch(opts.NewBranchName)
		if git.IsErrNotExist(err) {
			return nil, err
		}
		if newBranch != nil {
			return nil, ErrBranchAlreadyExists{opts.NewBranchName}
		}
	} else {
		if protected, _ := repo.IsProtectedBranchForPush(opts.BranchName, user); protected {
			return nil, ErrCannotCommit{UserName: user.LowerName}
		}
	}

	// Check that the path given in opts.Path is valid (not a git path)
	// and if an FromPath was given, to also check it
	newTreePath := cleanUploadFileName(opts.Path)
	if len(newTreePath) == 0 {
		return nil, ErrFilenameInvalid{opts.Path}
	}
	if opts.FromPath == "" {
		opts.FromPath = newTreePath
	}
	origTreePath := cleanUploadFileName(opts.FromPath)
	if len(opts.FromPath) > 0 && len(origTreePath) == 0 {
		return nil, ErrFilenameInvalid{opts.FromPath}
	}

	// Get the commit of the original branch
	commit, err := gitRepo.GetBranchCommit(opts.BranchName)
	if err != nil {
		return nil, err // Couldn't get a commit for the branch
	}

	// Check that the given existing path exists in the HEAD of the given BranchName if
	// SHA is given (meaning we are updating a file, not creating a new one)
	if opts.SHA != "" {
		if entry, err := commit.GetTreeEntryByPath(origTreePath); err != nil {
			if git.IsErrNotExist(err) {
				return nil, ErrRepoFileDoesNotExist{origTreePath}
			} else {
				return nil, err
			}
		} else {
			currentSHA := string(entry.ID[:])
			if currentSHA != opts.SHA {
				return nil, ErrShaDoesNotMatch{opts.SHA, currentSHA}
			}
		}
		// Check to see if we are needing to also move this updated file to a new file name
		// If so, we make sure the new file name doesn't already exist (cannot clobber)
		if origTreePath != newTreePath {
			if entry, err := commit.GetTreeEntryByPath(newTreePath); err != nil {
				if !git.IsErrNotExist(err) {
					return nil, err
				}
 			} else if entry != nil {
				return nil, ErrRepoFileAlreadyExist{newTreePath}
			}
		}
	}

	// For the path where this file will be created/updated, we need to make
	// sure no parts of the path are existing files or links except for the last
	// item in the path which is the file name
	treePathParts := strings.Split(newTreePath, "/")
	for index, part := range treePathParts {
		newTreePath = path.Join(newTreePath, part)
		entry, err := commit.GetTreeEntryByPath(newTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}
			return nil, err
		}
		if index < len(treePathParts)-1 {
			if !entry.IsDir() {
				return nil, ErrWithFilePath{fmt.Sprintf("%s is not a directory, it is a file", newTreePath)}
			}
		} else {
			if entry.IsLink() {
				return nil, ErrWithFilePath{fmt.Sprintf("%s is not a file, it is a symbolic link", newTreePath)}
			}
			if entry.IsDir() {
				return nil, ErrWithFilePath{fmt.Sprintf("%s is not a file, it is a directory", newTreePath)}
			}
		}
	}

	message := strings.TrimSpace(opts.Message)

	if err := uploader.UpdateRepoFile(repo, doer, &uploader.UpdateRepoFileOptions{
		LastCommitID: lastCommit,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		OldTreeName:  oldTreePath,
		NewTreeName:  form.TreePath,
		Message:      message,
		Content:      strings.Replace(form.Content, "\r", "", -1),
		IsNewFile:    isNewFile,
	}); err != nil {
		ctx.Data["Err_TreePath"] = true
		ctx.RenderWithErr(ctx.Tr("repo.editor.fail_to_update_file", form.TreePath, err), tplEditFile, &form)
		return
	}

	file := &File{}

	return file, nil
}

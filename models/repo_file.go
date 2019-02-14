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
	Branch    string
	Path      string
	OldPath   string
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

func CreateFile(doer, u *User, repo *Repository, gitRepo *git.Repository, opts FileOptions) (*File, error) {
	branch := "master"
	if opts.Branch != "" {
		branch = opts.Branch
	}
	if protected, _ := repo.IsProtectedBranchForPush(branch, doer); protected {
		return nil, ErrCannotCommit{UserName: doer.LowerName}
	}

	treePath := cleanUploadFileName(opts.Path)
	oldTreePath := opts.OldPath
	if len(treePath) == 0 {
		return nil, ErrFilenameInvalid{treePath}
	}

	if _, err := repo.GetBranch(branch); err == nil {
		return nil, err
	}

	commit, err := gitRepo.GetBranchCommit(branch)
	if err != nil {
		return nil, err
	}

	if opts.SHA != "" {
		if _, err := commit.GetTreeEntryByPath(treePath); err != nil {
			return nil, err
		}
	}

	var newTreePath string
	treePathParts := strings.Split(treePath, "/")
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

	if opts.SHA != "" {
		treeEntry, err := commit.GetTreeEntryByPath(oldTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				return nil, ErrRepoFileDoesNotExist{oldTreePath}
			} else {
				return nil, err
			}
		}
		if opts.SHA != string(treeEntry.ID[:]) {
			return nil, ErrShaDoesNotMatch{opts.SHA, string(treeEntry.ID[:])}
		}
	}

	if oldTreePath != treePath {
		// We have a new filename (rename or completely new file) so we need to make sure it doesn't already exist, can't clobber.
		entry, err := commit.GetTreeEntryByPath(treePath)
		if err != nil {
			if !git.IsErrNotExist(err) {
				return nil, err
			}
		}
		if entry != nil {
			return nil, ErrRepoFileAlreadyExist{treePath}
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

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"path"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/uploader"
)

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

// getParentTreeFields returns list of parent tree names and corresponding tree paths
// based on given tree path.
func getParentTreeFields(treePath string) (treeNames []string, treePaths []string) {
	if len(treePath) == 0 {
		return treeNames, treePaths
	}

	treeNames = strings.Split(treePath, "/")
	treePaths = make([]string, len(treeNames))
	for i := range treeNames {
		treePaths[i] = strings.Join(treeNames[:i+1], "/")
	}
	return treeNames, treePaths
}

func renderCommitRights(ctx *context.Context) bool {
	canCommit, err := ctx.Repo.CanCommitToBranch(ctx.User)
	if err != nil {
		log.Error(4, "CanCommitToBranch: %v", err)
	}
	ctx.Data["CanCommitToBranch"] = canCommit
	return canCommit
}

func modifyFile(ctx *context.Context, isNewFile bool) error {
	oldBranchName := ctx.Repo.BranchName
	branchName := oldBranchName
	oldTreePath := cleanUploadFileName(ctx.Repo.TreePath)
	canCommit := renderCommitRights(ctx)

	if len(oldTreePath) == 0 {
		return ErrFilenameInvalid{oldTreePath}
	}
	treeNames, treePaths := getParentTreeFields(ctx.Repo.TreePath)

	if oldBranchName != branchName {
		if _, err := ctx.Repo.Repository.GetBranch(branchName); err == nil {
			return err
		}
	} else if !canCommit {
		return ErrCannotCommit{UserName: ctx.User.LowerName}
	}

	var newTreePath string
	for index, part := range treeNames {
		newTreePath = path.Join(newTreePath, part)
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(newTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}
			return err
		}
		if index != len(treeNames)-1 {
			if !entry.IsDir() {
				return ErrWithFilePath{ctx.Tr("repo.editor.directory_is_a_file", part)}
			}
		} else {
			if entry.IsLink() {
				ctx.Tr("repo.editor.directory_is_a_file", part)
				ctx.RenderWithErr(ctx.Tr("repo.editor.file_is_a_symlink", part), tplEditFile, &form)
				return

			}
			if entry.IsDir() {
				ctx.Data["Err_TreePath"] = true
				ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_a_directory", part), tplEditFile, &form)
				return
			}
		}
	}

	if !isNewFile {
		_, err := ctx.Repo.Commit.GetTreeEntryByPath(oldTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				ctx.Data["Err_TreePath"] = true
				ctx.RenderWithErr(ctx.Tr("repo.editor.file_editing_no_longer_exists", oldTreePath), tplEditFile, &form)
			} else {
				ctx.ServerError("GetTreeEntryByPath", err)
			}
			return
		}
		if lastCommit != ctx.Repo.CommitID {
			files, err := ctx.Repo.Commit.GetFilesChangedSinceCommit(lastCommit)
			if err != nil {
				ctx.ServerError("GetFilesChangedSinceCommit", err)
				return
			}

			for _, file := range files {
				if file == form.TreePath {
					ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+lastCommit+"..."+ctx.Repo.CommitID), tplEditFile, &form)
					return
				}
			}
		}
	}

	if oldTreePath != form.TreePath {
		// We have a new filename (rename or completely new file) so we need to make sure it doesn't already exist, can't clobber.
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(form.TreePath)
		if err != nil {
			if !git.IsErrNotExist(err) {
				ctx.ServerError("GetTreeEntryByPath", err)
				return
			}
		}
		if entry != nil {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_already_exists", form.TreePath), tplEditFile, &form)
			return
		}
	}

	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		if isNewFile {
			message = ctx.Tr("repo.editor.add", form.TreePath)
		} else {
			message = ctx.Tr("repo.editor.update", form.TreePath)
		}
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if err := uploader.UpdateRepoFile(ctx.Repo.Repository, ctx.User, &uploader.UpdateRepoFileOptions{
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

	ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + branchName + "/" + strings.NewReplacer("%", "%25", "#", "%23", " ", "%20", "?", "%3F").Replace(form.TreePath))
}

// GetTags return repo's tags
func (repo *Repository) GetTags() ([]*git.Tag, error) {
	return GetTagsByPath(repo.RepoPath())
}

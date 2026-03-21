// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	context_service "code.gitea.io/gitea/services/context"
)

// getUniquePatchBranchName Gets a unique branch name for a new patch branch
// It will be in the form of <username>-patch-<num> where <num> is the first branch of this format
// that doesn't already exist. If we exceed 1000 tries or an error is thrown, we just return "" so the user has to
// type in the branch name themselves (will be an empty field)
func getUniquePatchBranchName(ctx context.Context, prefixName string, repo *repo_model.Repository) string {
	prefix := prefixName + "-patch-"
	for i := 1; i <= 1000; i++ {
		branchName := fmt.Sprintf("%s%d", prefix, i)
		if exist, err := git_model.IsBranchExist(ctx, repo.ID, branchName); err != nil {
			log.Error("getUniquePatchBranchName: %v", err)
			return ""
		} else if !exist {
			return branchName
		}
	}
	return ""
}

// getClosestParentWithFiles Recursively gets the closest path of parent in a tree that has files when a file in a tree is
// deleted. It returns "" for the tree root if no parents other than the root have files.
func getClosestParentWithFiles(gitRepo *git.Repository, branchName, originTreePath string) string {
	var f func(treePath string, commit *git.Commit) string
	f = func(treePath string, commit *git.Commit) string {
		if treePath == "" || treePath == "." {
			return ""
		}
		// see if the tree has entries
		if tree, err := commit.SubTree(treePath); err != nil {
			return f(path.Dir(treePath), commit) // failed to get the tree, going up a dir
		} else if entries, err := tree.ListEntries(); err != nil || len(entries) == 0 {
			return f(path.Dir(treePath), commit) // no files in this dir, going up a dir
		}
		return treePath
	}
	commit, err := gitRepo.GetBranchCommit(branchName) // must get the commit again to get the latest change
	if err != nil {
		log.Error("GetBranchCommit: %v", err)
		return ""
	}
	return f(originTreePath, commit)
}

// CodeEditorConfig is also used by frontend, defined in "codeeditor.ts"
type CodeEditorConfig struct {
	PreviewableExtensions []string `json:"previewable_extensions"`
	LineWrapExtensions    []string `json:"line_wrap_extensions"`
	LineWrapOn            bool     `json:"line_wrap_on"`

	IndentStyle            string `json:"indent_style"`
	IndentSize             int    `json:"indent_size"`
	TabWidth               int    `json:"tab_width"`
	TrimTrailingWhitespace *bool  `json:"trim_trailing_whitespace,omitempty"`
}

func getCodeEditorConfig(ctx *context_service.Context, treePath string) (ret CodeEditorConfig) {
	ret.PreviewableExtensions = markup.PreviewableExtensions()
	ret.LineWrapExtensions = setting.Repository.Editor.LineWrapExtensions
	ret.LineWrapOn = util.SliceContainsString(ret.LineWrapExtensions, path.Ext(treePath), true)
	ec, _, err := ctx.Repo.GetEditorconfig()
	if err == nil {
		def, err := ec.GetDefinitionForFilename(treePath)
		if err == nil {
			ret.IndentStyle = def.IndentStyle
			ret.IndentSize, _ = strconv.Atoi(def.IndentSize)
			ret.TabWidth = def.TabWidth
			ret.TrimTrailingWhitespace = def.TrimTrailingWhitespace
		}
	}
	return ret
}

// getParentTreeFields returns list of parent tree names and corresponding tree paths based on given treePath.
// eg: []{"a", "b", "c"}, []{"a", "a/b", "a/b/c"}
// or: []{""}, []{""} for the root treePath
func getParentTreeFields(treePath string) (treeNames, treePaths []string) {
	treeNames = strings.Split(treePath, "/")
	treePaths = make([]string, len(treeNames))
	for i := range treeNames {
		treePaths[i] = strings.Join(treeNames[:i+1], "/")
	}
	return treeNames, treePaths
}

// getUniqueRepositoryName Gets a unique repository name for a user
// It will append a -<num> postfix if the name is already taken
func getUniqueRepositoryName(ctx context.Context, ownerID int64, name string) string {
	uniqueName := name
	for i := 1; i < 1000; i++ {
		_, err := repo_model.GetRepositoryByName(ctx, ownerID, uniqueName)
		if err != nil || repo_model.IsErrRepoNotExist(err) {
			return uniqueName
		}
		uniqueName = fmt.Sprintf("%s-%d", name, i)
		i++
	}
	return ""
}

func editorPushBranchToForkedRepository(ctx context.Context, doer *user_model.User, baseRepo *repo_model.Repository, baseBranchName string, targetRepo *repo_model.Repository, targetBranchName string) error {
	return gitrepo.Push(ctx, baseRepo, targetRepo, git.PushOptions{
		Branch: baseBranchName + ":" + targetBranchName,
		Env:    repo_module.PushingEnvironment(doer, targetRepo),
	})
}

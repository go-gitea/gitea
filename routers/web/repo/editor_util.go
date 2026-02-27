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

// CodeEditorPhrases returns a map of CodeMirror phrase keys to translated strings.
// The keys must match the exact phrases used by CodeMirror's built-in extensions.
func CodeEditorPhrases(ctx *context_service.Context) map[string]string {
	return map[string]string{
		"Find":                     ctx.Locale.TrString("editor.code_editor.find"),
		"Replace":                  ctx.Locale.TrString("editor.code_editor.replace"),
		"next":                     ctx.Locale.TrString("editor.code_editor.next"),
		"previous":                 ctx.Locale.TrString("editor.code_editor.previous"),
		"all":                      ctx.Locale.TrString("editor.code_editor.all"),
		"match case":               ctx.Locale.TrString("editor.code_editor.match_case"),
		"regexp":                   ctx.Locale.TrString("editor.code_editor.regexp"),
		"by word":                  ctx.Locale.TrString("editor.code_editor.by_word"),
		"replace":                  ctx.Locale.TrString("editor.code_editor.replace_one"),
		"replace all":              ctx.Locale.TrString("editor.code_editor.replace_all"),
		"close":                    ctx.Locale.TrString("editor.code_editor.close"),
		"current match":            ctx.Locale.TrString("editor.code_editor.current_match"),
		"on line":                  ctx.Locale.TrString("editor.code_editor.on_line"),
		"Go to line":               ctx.Locale.TrString("editor.code_editor.go_to_line"),
		"go":                       ctx.Locale.TrString("editor.code_editor.go"),
		"replaced match on line $": ctx.Locale.TrString("editor.code_editor.replaced_match_on_line"),
		"replaced $ matches":       ctx.Locale.TrString("editor.code_editor.replaced_matches"),
		"Control character":        ctx.Locale.TrString("editor.code_editor.control_character"),
		"Completions":              ctx.Locale.TrString("editor.code_editor.completions"),
		"Folded lines":             ctx.Locale.TrString("editor.code_editor.folded_lines"),
		"Unfolded lines":           ctx.Locale.TrString("editor.code_editor.unfolded_lines"),
		"to":                       ctx.Locale.TrString("editor.code_editor.to"),
		"folded code":              ctx.Locale.TrString("editor.code_editor.folded_code"),
		"unfold":                   ctx.Locale.TrString("editor.code_editor.unfold"),
		"Fold line":                ctx.Locale.TrString("editor.code_editor.fold_line"),
		"Unfold line":              ctx.Locale.TrString("editor.code_editor.unfold_line"),
		"Selection deleted":        ctx.Locale.TrString("editor.code_editor.selection_deleted"),

		// command palette
		"Type a command...":        ctx.Locale.TrString("editor.code_editor.type_a_command"),
		"Undo":                     ctx.Locale.TrString("editor.code_editor.undo"),
		"Redo":                     ctx.Locale.TrString("editor.code_editor.redo"),
		"Select All":               ctx.Locale.TrString("editor.code_editor.select_all"),
		"Delete Line":              ctx.Locale.TrString("editor.code_editor.delete_line"),
		"Move Line Up":             ctx.Locale.TrString("editor.code_editor.move_line_up"),
		"Move Line Down":           ctx.Locale.TrString("editor.code_editor.move_line_down"),
		"Copy Line Up":             ctx.Locale.TrString("editor.code_editor.copy_line_up"),
		"Copy Line Down":           ctx.Locale.TrString("editor.code_editor.copy_line_down"),
		"Toggle Comment":           ctx.Locale.TrString("editor.code_editor.toggle_comment"),
		"Insert Blank Line":        ctx.Locale.TrString("editor.code_editor.insert_blank_line"),
		"Add Cursor Above":         ctx.Locale.TrString("editor.code_editor.add_cursor_above"),
		"Add Cursor Below":         ctx.Locale.TrString("editor.code_editor.add_cursor_below"),
		"Add Next Occurrence":      ctx.Locale.TrString("editor.code_editor.add_next_occurrence"),
		"Go to Matching Bracket":   ctx.Locale.TrString("editor.code_editor.go_to_matching_bracket"),
		"Indent More":              ctx.Locale.TrString("editor.code_editor.indent_more"),
		"Indent Less":              ctx.Locale.TrString("editor.code_editor.indent_less"),
		"Fold Code":                ctx.Locale.TrString("editor.code_editor.fold_code"),
		"Unfold Code":              ctx.Locale.TrString("editor.code_editor.unfold_code"),
		"Fold All":                 ctx.Locale.TrString("editor.code_editor.fold_all"),
		"Unfold All":               ctx.Locale.TrString("editor.code_editor.unfold_all"),
		"Trigger Autocomplete":     ctx.Locale.TrString("editor.code_editor.trigger_autocomplete"),
		"Trim Trailing Whitespace": ctx.Locale.TrString("editor.code_editor.trim_trailing_whitespace"),
	}
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

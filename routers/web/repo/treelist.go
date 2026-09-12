// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"
	"net/http"
	"strings"

	pull_model "gitea.dev/models/pull"
	"gitea.dev/modules/base"
	"gitea.dev/modules/fileicon"
	"gitea.dev/modules/git"
	"gitea.dev/services/context"
	"gitea.dev/services/gitdiff"
	files_service "gitea.dev/services/repository/files"

	"github.com/go-enry/go-enry/v2"
)

// TreeList get all files' entries of a repository
func TreeList(ctx *context.Context) {
	tree, err := ctx.Repo.Commit.SubTree(ctx, ctx.Repo.GitRepo, "/")
	if err != nil {
		ctx.ServerError("Repo.Commit.SubTree", err)
		return
	}

	entries, err := tree.ListEntriesRecursiveFast(ctx, ctx.Repo.GitRepo)
	if err != nil {
		ctx.ServerError("ListEntriesRecursiveFast", err)
		return
	}
	entries.CustomSort(base.NaturalSortCompare)

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !isExcludedEntry(entry) {
			files = append(files, entry.Name())
		}
	}
	ctx.JSON(http.StatusOK, files)
}

func isExcludedEntry(entry *git.TreeEntry) bool {
	if entry.IsDir() {
		return true
	}

	if entry.IsSubModule() {
		return true
	}

	if enry.IsVendor(entry.Name()) {
		return true
	}

	return false
}

// WebDiffFileItem is used by frontend, check the field names in frontend before changing
type WebDiffFileItem struct {
	Name       string
	OldPath    string             `json:",omitzero"`
	DiffStatus string             `json:",omitzero"`
	IsViewed   bool               `json:",omitzero"`
	Children   []*WebDiffFileItem `json:",omitzero"`
	IconID     string             `json:",omitzero"`
	IconClass  string             `json:",omitzero"` // only when it differs from WebDiffFileTree.FileIconClass
}

// WebDiffFileTree is used by frontend, check the field names in frontend before changing
type WebDiffFileTree struct {
	TreeRoot       WebDiffFileItem
	FolderIcon     template.HTML
	FolderOpenIcon template.HTML
	FileIconClass  string // the class nearly every file uses, items only carry the exceptions
}

func setDiffFileTreeData(ctx *context.Context, diffTree *gitdiff.DiffTree, filesViewedState map[string]pull_model.ViewedState) {
	renderedIconPool := fileicon.NewRenderedIconPool()
	ctx.Data["DiffFileTree"] = transformDiffTreeForWeb(renderedIconPool, diffTree, filesViewedState)
	ctx.Data["FileIconPoolHTML"] = renderedIconPool.RenderToHTML()
}

// transformDiffTreeForWeb transforms a gitdiff.DiffTree into a WebDiffFileTree for Web UI rendering
// it also takes a map of file names to their viewed state, which is used to mark files as viewed
func transformDiffTreeForWeb(renderedIconPool *fileicon.RenderedIconPool, diffTree *gitdiff.DiffTree, filesViewedState map[string]pull_model.ViewedState) (dft WebDiffFileTree) {
	dft.FolderIcon = fileicon.RenderEntryIconHTML(renderedIconPool, fileicon.EntryInfoFolder())
	dft.FolderOpenIcon = fileicon.RenderEntryIconHTML(renderedIconPool, fileicon.EntryInfoFolderOpen())

	dirNodes := map[string]*WebDiffFileItem{"": &dft.TreeRoot}
	addItem := func(path string, item *WebDiffFileItem) {
		var parentPath string
		if dir, name, found := strings.CutLast(path, "/"); found {
			parentPath, item.Name = dir, name
		} else {
			item.Name = path
		}
		parentNode, parentExists := dirNodes[parentPath]
		if !parentExists {
			parentNode = &dft.TreeRoot
			fields := strings.Split(parentPath, "/")
			for idx, field := range fields {
				nodePath := strings.Join(fields[:idx+1], "/")
				node, ok := dirNodes[nodePath]
				if !ok {
					node = &WebDiffFileItem{Name: field}
					dirNodes[nodePath] = node
					parentNode.Children = append(parentNode.Children, node)
				}
				parentNode = node
			}
		}
		parentNode.Children = append(parentNode.Children, item)
	}

	for _, file := range diffTree.Files {
		item := &WebDiffFileItem{DiffStatus: file.Status}
		if file.BasePath != file.HeadPath {
			item.OldPath = file.BasePath
		}
		item.IsViewed = filesViewedState[file.HeadPath] == pull_model.Viewed
		addItem(file.HeadPath, item)
		iconID, class := fileicon.RenderEntryIconID(renderedIconPool, &fileicon.EntryInfo{BaseName: item.Name, EntryMode: file.HeadMode})
		item.IconID = iconID
		if dft.FileIconClass == "" {
			dft.FileIconClass = class
		} else if class != dft.FileIconClass {
			item.IconClass = class
		}
	}
	for _, node := range dft.TreeRoot.Children {
		for len(node.Children) == 1 && node.Children[0].Children != nil {
			child := node.Children[0]
			node.Name += "/" + child.Name
			node.Children = child.Children
		}
	}
	return dft
}

func TreeViewNodes(ctx *context.Context) {
	renderedIconPool := fileicon.NewRenderedIconPool()
	results, err := files_service.GetTreeViewNodes(ctx, ctx.Repo.RepoLink, renderedIconPool, ctx.Repo.GitRepo, ctx.Repo.Commit, ctx.Repo.TreePath, ctx.FormString("sub_path"))
	if err != nil {
		ctx.ServerError("GetTreeViewNodes", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"fileTreeNodes": results, "renderedIconPool": renderedIconPool.IconSVGs})
}

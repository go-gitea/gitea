// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"html/template"
	"net/url"
	"path"
	"sort"
	"strings"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/base"
	"gitea.dev/modules/fileicon"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// GetTreeBySHA get the GitTreeResponse of a repository using a sha hash (id of a commit or a tree)
func GetTreeBySHA(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, sha string, page, perPage int, recursive bool) (*api.GitTreeResponse, error) {
	gitTree, err := gitRepo.GetTree(sha)
	if err != nil {
		return nil, util.NewInvalidArgumentErrorf("sha not found [%s]", sha)
	}
	tree := new(api.GitTreeResponse)
	tree.SHA = gitTree.ID.String() // always return the real tree id to end users, but not the commit's id if sha is a commit
	tree.URL = repo.APIURL(ctx) + "/git/trees/" + url.PathEscape(tree.SHA)
	var entries git.Entries
	if recursive {
		entries, err = gitTree.ListEntriesRecursiveWithSize(ctx, gitRepo)
	} else {
		entries, err = gitTree.ListEntries(ctx, gitRepo)
	}
	if err != nil {
		return nil, err
	}
	apiURL := repo.APIURL()
	blobURLBase := apiURL + "/git/blobs/"
	treeURLBase := apiURL + "/git/trees/"

	if perPage <= 0 || perPage > setting.API.DefaultGitTreesPerPage {
		perPage = setting.API.DefaultGitTreesPerPage
	}
	page = max(page, 1)

	tree.Page = page
	tree.TotalCount = len(entries)
	rangeStart := perPage * (page - 1) // int might overflow
	if rangeStart < 0 || rangeStart >= len(entries) {
		return tree, nil
	}
	rangeEnd := min(rangeStart+perPage, len(entries))
	tree.Truncated = rangeEnd < len(entries)
	tree.Entries = make([]api.GitEntry, rangeEnd-rangeStart)
	for e := rangeStart; e < rangeEnd; e++ {
		i := e - rangeStart

		tree.Entries[i].Path = entries[e].Name()
		tree.Entries[i].Mode = fmt.Sprintf("%06o", entries[e].Mode())
		tree.Entries[i].Type = entries[e].Type()
		tree.Entries[i].Size = entries[e].GetSize(ctx, gitRepo)
		tree.Entries[i].SHA = entries[e].ID.String()

		if entries[e].IsDir() {
			tree.Entries[i].URL = treeURLBase + entries[e].ID.String()
		} else if entries[e].IsSubModule() {
			// In GitHub Rest API Version=2022-11-28, if a tree entry is a submodule,
			// its url will be returned as an empty string.
			// So the URL will be set to "" here.
			tree.Entries[i].URL = ""
		} else {
			tree.Entries[i].URL = blobURLBase + entries[e].ID.String()
		}
	}
	return tree, nil
}

func entryModeString(entryMode git.EntryMode) string {
	switch entryMode {
	case git.EntryModeBlob:
		return "blob"
	case git.EntryModeExec:
		return "exec"
	case git.EntryModeSymlink:
		return "symlink"
	case git.EntryModeCommit:
		return "commit" // submodule
	case git.EntryModeTree:
		return "tree"
	}
	return "unknown"
}

type TreeViewNode struct {
	EntryName     string        `json:"entryName"`
	EntryMode     string        `json:"entryMode"`
	EntryIcon     template.HTML `json:"entryIcon"`
	EntryIconOpen template.HTML `json:"entryIconOpen,omitempty"`

	SymLinkedToMode string `json:"symLinkedToMode,omitempty"` // TODO: for the EntryMode="symlink"

	FullPath     string          `json:"fullPath"`
	SubmoduleURL string          `json:"submoduleUrl,omitempty"`
	Children     []*TreeViewNode `json:"children,omitempty"`
}

func (node *TreeViewNode) sortLevel() int {
	return util.Iif(node.EntryMode == "tree" || node.EntryMode == "commit", 0, 1)
}

func newTreeViewNodeFromEntry(ctx context.Context, repoLink string, renderedIconPool *fileicon.RenderedIconPool, gitRepo *git.Repository, commit *git.Commit, parentDir string, entry *git.TreeEntry) *TreeViewNode {
	node := &TreeViewNode{
		EntryName: entry.Name(),
		EntryMode: entryModeString(entry.Mode()),
		FullPath:  path.Join(parentDir, entry.Name()),
	}

	entryInfo := fileicon.EntryInfoFromGitTreeEntry(ctx, gitRepo, commit, node.FullPath, entry)
	node.EntryIcon = fileicon.RenderEntryIconHTML(renderedIconPool, entryInfo)
	if entryInfo.EntryMode.IsDir() {
		entryInfo.IsOpen = true
		node.EntryIconOpen = fileicon.RenderEntryIconHTML(renderedIconPool, entryInfo)
	}

	if node.EntryMode == "commit" {
		if subModule, err := commit.GetSubModule(ctx, gitRepo, node.FullPath); err != nil {
			log.Error("GetSubModule: %v", err)
		} else if subModule != nil {
			submoduleFile := git.NewCommitSubmoduleFile(repoLink, node.FullPath, subModule.URL, entry.ID.String())
			webLink := submoduleFile.SubmoduleWebLinkTree(ctx)
			if webLink != nil {
				node.SubmoduleURL = webLink.CommitWebLink
			}
		}
	}

	return node
}

// sortTreeViewNodes list directory first and with alpha sequence
func sortTreeViewNodes(nodes []*TreeViewNode) {
	sort.Slice(nodes, func(i, j int) bool {
		a, b := nodes[i].sortLevel(), nodes[j].sortLevel()
		if a != b {
			return a < b
		}
		return base.NaturalSortCompare(nodes[i].EntryName, nodes[j].EntryName) < 0
	})
}

func listTreeNodes(ctx context.Context, repoLink string, renderedIconPool *fileicon.RenderedIconPool, gitRepo *git.Repository, commit *git.Commit, tree *git.Tree, treePath, subPath string) ([]*TreeViewNode, error) {
	entries, err := tree.ListEntries(ctx, gitRepo)
	if err != nil {
		return nil, err
	}

	subPathDirName, subPathRemaining, _ := strings.Cut(subPath, "/")
	nodes := make([]*TreeViewNode, 0, len(entries))
	for _, entry := range entries {
		node := newTreeViewNodeFromEntry(ctx, repoLink, renderedIconPool, gitRepo, commit, treePath, entry)
		nodes = append(nodes, node)
		if entry.IsDir() && subPathDirName == entry.Name() {
			subTreePath := treePath + "/" + node.EntryName
			if subTreePath[0] == '/' {
				subTreePath = subTreePath[1:]
			}
			subNodes, err := listTreeNodes(ctx, repoLink, renderedIconPool, gitRepo, commit, entry.Tree(gitRepo), subTreePath, subPathRemaining)
			if err != nil {
				log.Error("listTreeNodes: %v", err)
			} else {
				node.Children = subNodes
			}
		}
	}
	sortTreeViewNodes(nodes)
	return nodes, nil
}

func GetTreeViewNodes(ctx context.Context, repoLink string, renderedIconPool *fileicon.RenderedIconPool, gitRepo *git.Repository, commit *git.Commit, treePath, subPath string) ([]*TreeViewNode, error) {
	entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, treePath)
	if err != nil {
		return nil, err
	}
	return listTreeNodes(ctx, repoLink, renderedIconPool, gitRepo, commit, entry.Tree(gitRepo), treePath, subPath)
}

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

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/fileicon"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

// ErrSHANotFound represents a "SHADoesNotMatch" kind of error.
type ErrSHANotFound struct {
	SHA string
}

// IsErrSHANotFound checks if an error is a ErrSHANotFound.
func IsErrSHANotFound(err error) bool {
	_, ok := err.(ErrSHANotFound)
	return ok
}

func (err ErrSHANotFound) Error() string {
	return fmt.Sprintf("sha not found [%s]", err.SHA)
}

func (err ErrSHANotFound) Unwrap() error {
	return util.ErrNotExist
}

// GetTreeBySHA get the GitTreeResponse of a repository using a sha hash.
func GetTreeBySHA(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, sha string, page, perPage int, recursive bool) (*api.GitTreeResponse, error) {
	gitTree, err := gitRepo.GetTree(sha)
	if err != nil || gitTree == nil {
		return nil, ErrSHANotFound{ // TODO: this error has never been catch outside of this function
			SHA: sha,
		}
	}
	tree := new(api.GitTreeResponse)
	tree.SHA = gitTree.ResolvedID.String()
	tree.URL = repo.APIURL() + "/git/trees/" + url.PathEscape(tree.SHA)
	var entries git.Entries
	if recursive {
		entries, err = gitTree.ListEntriesRecursiveWithSize()
	} else {
		entries, err = gitTree.ListEntries()
	}
	if err != nil {
		return nil, err
	}
	apiURL := repo.APIURL()
	apiURLLen := len(apiURL)
	objectFormat := git.ObjectFormatFromName(repo.ObjectFormatName)
	hashLen := objectFormat.FullLength()

	const gitBlobsPath = "/git/blobs/"
	blobURL := make([]byte, apiURLLen+hashLen+len(gitBlobsPath))
	copy(blobURL, apiURL)
	copy(blobURL[apiURLLen:], []byte(gitBlobsPath))

	const gitTreePath = "/git/trees/"
	treeURL := make([]byte, apiURLLen+hashLen+len(gitTreePath))
	copy(treeURL, apiURL)
	copy(treeURL[apiURLLen:], []byte(gitTreePath))

	// copyPos is at the start of the hash
	copyPos := len(treeURL) - hashLen

	if perPage <= 0 || perPage > setting.API.DefaultGitTreesPerPage {
		perPage = setting.API.DefaultGitTreesPerPage
	}
	if page <= 0 {
		page = 1
	}
	tree.Page = page
	tree.TotalCount = len(entries)
	rangeStart := perPage * (page - 1)
	if rangeStart >= len(entries) {
		return tree, nil
	}
	var rangeEnd int
	if len(entries) > perPage {
		tree.Truncated = true
	}
	if rangeStart+perPage < len(entries) {
		rangeEnd = rangeStart + perPage
	} else {
		rangeEnd = len(entries)
	}
	tree.Entries = make([]api.GitEntry, rangeEnd-rangeStart)
	for e := rangeStart; e < rangeEnd; e++ {
		i := e - rangeStart

		tree.Entries[i].Path = entries[e].Name()
		tree.Entries[i].Mode = fmt.Sprintf("%06o", entries[e].Mode())
		tree.Entries[i].Type = entries[e].Type()
		tree.Entries[i].Size = entries[e].Size()
		tree.Entries[i].SHA = entries[e].ID.String()

		if entries[e].IsDir() {
			copy(treeURL[copyPos:], entries[e].ID.String())
			tree.Entries[i].URL = string(treeURL)
		} else if entries[e].IsSubModule() {
			// In Github Rest API Version=2022-11-28, if a tree entry is a submodule,
			// its url will be returned as an empty string.
			// So the URL will be set to "" here.
			tree.Entries[i].URL = ""
		} else {
			copy(blobURL[copyPos:], entries[e].ID.String())
			tree.Entries[i].URL = string(blobURL)
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

func newTreeViewNodeFromEntry(ctx context.Context, renderedIconPool *fileicon.RenderedIconPool, commit *git.Commit, parentDir string, entry *git.TreeEntry) *TreeViewNode {
	node := &TreeViewNode{
		EntryName: entry.Name(),
		EntryMode: entryModeString(entry.Mode()),
		FullPath:  path.Join(parentDir, entry.Name()),
	}

	if entry.IsLink() {
		// TODO: symlink to a folder or a file, the icon differs
		target, err := entry.FollowLink()
		if err == nil {
			_ = target.IsDir()
			// if target.IsDir() { } else { }
		}
	}

	if node.EntryIcon == "" {
		node.EntryIcon = fileicon.RenderEntryIcon(renderedIconPool, entry)
		// TODO: no open icon support yet
		// node.EntryIconOpen = fileicon.RenderEntryIconOpen(renderedIconPool, entry)
	}

	if node.EntryMode == "commit" {
		if subModule, err := commit.GetSubModule(node.FullPath); err != nil {
			log.Error("GetSubModule: %v", err)
		} else if subModule != nil {
			submoduleFile := git.NewCommitSubmoduleFile(subModule.URL, entry.ID.String())
			webLink := submoduleFile.SubmoduleWebLink(ctx)
			node.SubmoduleURL = webLink.CommitWebLink
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
		return nodes[i].EntryName < nodes[j].EntryName
	})
}

func listTreeNodes(ctx context.Context, renderedIconPool *fileicon.RenderedIconPool, commit *git.Commit, tree *git.Tree, treePath, subPath string) ([]*TreeViewNode, error) {
	entries, err := tree.ListEntries()
	if err != nil {
		return nil, err
	}

	subPathDirName, subPathRemaining, _ := strings.Cut(subPath, "/")
	nodes := make([]*TreeViewNode, 0, len(entries))
	for _, entry := range entries {
		node := newTreeViewNodeFromEntry(ctx, renderedIconPool, commit, treePath, entry)
		nodes = append(nodes, node)
		if entry.IsDir() && subPathDirName == entry.Name() {
			subTreePath := treePath + "/" + node.EntryName
			if subTreePath[0] == '/' {
				subTreePath = subTreePath[1:]
			}
			subNodes, err := listTreeNodes(ctx, renderedIconPool, commit, entry.Tree(), subTreePath, subPathRemaining)
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

func GetTreeViewNodes(ctx context.Context, renderedIconPool *fileicon.RenderedIconPool, commit *git.Commit, treePath, subPath string) ([]*TreeViewNode, error) {
	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}
	return listTreeNodes(ctx, renderedIconPool, commit, entry.Tree(), treePath, subPath)
}

// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
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
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Path         string          `json:"path"`
	SubModuleURL string          `json:"sub_module_url,omitempty"`
	Children     []*TreeViewNode `json:"children,omitempty"`
}

func (node *TreeViewNode) sortLevel() int {
	switch node.Type {
	case "tree", "commit":
		return 0
	default:
		return 1
	}
}

func newTreeViewNodeFromEntry(ctx context.Context, commit *git.Commit, parentDir string, entry *git.TreeEntry) *TreeViewNode {
	node := &TreeViewNode{
		Name: entry.Name(),
		Type: entryModeString(entry.Mode()),
		Path: path.Join(parentDir, entry.Name()),
	}

	if node.Type == "commit" {
		if subModule, err := commit.GetSubModule(node.Path); err != nil {
			log.Error("GetSubModule: %v", err)
		} else if subModule != nil {
			submoduleFile := git.NewCommitSubmoduleFile(subModule.URL, entry.ID.String())
			webLink := submoduleFile.SubmoduleWebLink(ctx)
			node.SubModuleURL = webLink.CommitWebLink
		}
	}

	return node
}

// sortTreeViewNodes list directory first and with alpha sequence
func sortTreeViewNodes(nodes []*TreeViewNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].sortLevel() != nodes[j].sortLevel() {
			return nodes[i].sortLevel() < nodes[j].sortLevel()
		}
		return nodes[i].Name < nodes[j].Name
	})
}

/*
Example 1: (path: /)

	GET /repo/name/tree/

	resp:
	[{
	    "name": "d1",
	    "isFile": false,
	    "path": "d1"
	},{
	    "name": "d2",
	    "isFile": false,
	    "path": "d2"
	},{
	    "name": "d3",
	    "isFile": false,
	    "path": "d3"
	},{
	    "name": "f1",
	    "isFile": true,
	    "path": "f1"
	},]

Example 2: (path: d3)

	GET /repo/name/tree/d3
	resp:
	[{
	    "name": "d3d1",
	    "isFile": false,
	    "path": "d3/d3d1"
	}]

Example 3: (path: d3/d3d1)

	GET /repo/name/tree/d3/d3d1
	resp:
	[{
	    "name": "d3d1f1",
	    "isFile": true,
	    "path": "d3/d3d1/d3d1f1"
	},{
	    "name": "d3d1f1",
	    "isFile": true,
	    "path": "d3/d3d1/d3d1f2"
	}]
*/
func GetTreeList(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, treePath string, ref git.RefName, recursive bool) ([]*TreeViewNode, error) {
	if repo.IsEmpty {
		return nil, nil
	}
	if ref == "" {
		ref = git.RefNameFromBranch(repo.DefaultBranch)
	}

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref.String())
	if err != nil {
		return nil, err
	}

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	// If the entry is a file, we return a FileContentResponse object
	if entry.Type() != "tree" {
		return nil, fmt.Errorf("%s is not a tree", treePath)
	}

	gitTree, err := commit.SubTree(treePath)
	if err != nil {
		return nil, err
	}
	var entries git.Entries
	if recursive {
		entries, err = gitTree.ListEntriesRecursiveFast()
	} else {
		entries, err = gitTree.ListEntries()
	}
	if err != nil {
		return nil, err
	}

	var treeViewNodes []*TreeViewNode
	mapTree := make(map[string][]*TreeViewNode)
	for _, e := range entries {
		subTreePath := path.Join(treePath, e.Name())

		if strings.Contains(e.Name(), "/") {
			mapTree[path.Dir(e.Name())] = append(mapTree[path.Dir(e.Name())], &TreeViewNode{
				Name: path.Base(e.Name()),
				Type: entryModeString(e.Mode()),
				Path: subTreePath,
			})
		} else {
			treeViewNodes = append(treeViewNodes, &TreeViewNode{
				Name: e.Name(),
				Type: entryModeString(e.Mode()),
				Path: subTreePath,
			})
		}
	}

	for _, node := range treeViewNodes {
		if node.Type == "tree" {
			node.Children = mapTree[node.Path]
			sortTreeViewNodes(node.Children)
		}
	}

	sortTreeViewNodes(treeViewNodes)

	return treeViewNodes, nil
}

// GetTreeInformation returns the first level directories and files and all the trees of the path to treePath.
// If treePath is a directory, list all subdirectories and files of treePath.
/*
Example 1: (path: /)
    GET /repo/name/tree/?recursive=true
    resp:
    [{
        "name": "d1",
        "isFile": false,
        "path": "d1"
    },{
        "name": "d2",
        "isFile": false,
        "path": "d2"
    },{
        "name": "d3",
        "isFile": false,
        "path": "d3"
    },{
        "name": "f1",
        "isFile": true,
        "path": "f1"
    },]

Example 2: (path: d3)
    GET /repo/name/tree/d3?recursive=true
    resp:
    [{
        "name": "d1",
        "isFile": false,
        "path": "d1"
    },{
        "name": "d2",
        "isFile": false,
        "path": "d2"
    },{
        "name": "d3",
        "isFile": false,
        "path": "d3",
        "children": [{
            "name": "d3d1",
            "isFile": false,
            "path": "d3/d3d1"
        }]
    },{
        "name": "f1",
        "isFile": true,
        "path": "f1"
    },]

Example 3: (path: d3/d3d1)
    GET /repo/name/tree/d3/d3d1?recursive=true
    resp:
    [{
        "name": "d1",
        "isFile": false,
        "path": "d1"
    },{
        "name": "d2",
        "isFile": false,
        "path": "d2"
    },{
        "name": "d3",
        "isFile": false,
        "path": "d3",
        "children": [{
            "name": "d3d1",
            "isFile": false,
            "path": "d3/d3d1",
            "children": [{
                "name": "d3d1f1",
                "isFile": true,
                "path": "d3/d3d1/d3d1f1"
            },{
                "name": "d3d1f1",
                "isFile": true,
                "path": "d3/d3d1/d3d1f2"
            }]
        }]
    },{
        "name": "f1",
        "isFile": true,
        "path": "f1"
    },]

Example 4: (path: d2/d2f1)
    GET /repo/name/tree/d2/d2f1?recursive=true
    resp:
    [{
        "name": "d1",
        "isFile": false,
        "path": "d1"
    },{
        "name": "d2",
        "isFile": false,
        "path": "d2",
        "children": [{
            "name": "d2f1",
            "isFile": true,
            "path": "d2/d2f1"
        }]
    },{
        "name": "d3",
        "isFile": false,
        "path": "d3"
    },{
        "name": "f1",
        "isFile": true,
        "path": "f1"
    },]
*/
func GetTreeInformation(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, treePath string, ref git.RefName) ([]*TreeViewNode, error) {
	if repo.IsEmpty {
		return nil, nil
	}
	if ref == "" {
		ref = git.RefNameFromBranch(repo.DefaultBranch)
	}

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref.String())
	if err != nil {
		return nil, err
	}

	// get root entries
	rootEntries, err := commit.ListEntries()
	if err != nil {
		return nil, err
	}

	dir := treePath
	if dir != "" {
		lastDirEntry, err := commit.GetTreeEntryByPath(treePath)
		if err != nil {
			return nil, err
		}
		if lastDirEntry.IsRegular() {
			// path.Dir cannot correctly handle .xxx file
			dir, _ = path.Split(treePath)
			dir = strings.TrimRight(dir, "/")
		}
	}

	treeViewNodes := make([]*TreeViewNode, 0, len(rootEntries))
	fields := strings.Split(dir, "/")
	var parentNode *TreeViewNode
	for _, entry := range rootEntries {
		node := newTreeViewNodeFromEntry(ctx, commit, "", entry)
		treeViewNodes = append(treeViewNodes, node)
		if dir != "" && fields[0] == entry.Name() {
			parentNode = node
		}
	}

	sortTreeViewNodes(treeViewNodes)
	if dir == "" || parentNode == nil {
		return treeViewNodes, nil
	}

	for i := 1; i < len(fields); i++ {
		parentNode.Children = []*TreeViewNode{
			{
				Name: fields[i],
				Type: "tree",
				Path: path.Join(fields[:i+1]...),
			},
		}
		parentNode = parentNode.Children[0]
	}

	tree, err := commit.Tree.SubTree(dir)
	if err != nil {
		return nil, err
	}
	entries, err := tree.ListEntries()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		parentNode.Children = append(parentNode.Children, newTreeViewNodeFromEntry(ctx, commit, dir, entry))
	}
	sortTreeViewNodes(parentNode.Children)
	return treeViewNodes, nil
}

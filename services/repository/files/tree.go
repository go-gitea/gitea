// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"net/url"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
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

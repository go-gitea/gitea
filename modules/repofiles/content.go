// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

// ContentType repo content type
type ContentType string

// The string representations of different content types
const (
	// ContentTypeRegular regular content type (file)
	ContentTypeRegular ContentType = "file"
	// ContentTypeDir dir content type (dir)
	ContentTypeDir ContentType = "dir"
	// ContentLink link content type (symlink)
	ContentTypeLink ContentType = "symlink"
	// ContentTag submodule content type (submodule)
	ContentTypeSubmodule ContentType = "submodule"
)

// String gets the string of ContentType
func (ct *ContentType) String() string {
	return string(*ct)
}

// GetFileContentsOrList gets the meta data of a file's contents (*FileContentsResponse) if treePath not a tree
// directory, otherwise a listing of file contents ([]*FileContentsResponse). Ref can be a branch, commit or tag
func GetFileContentsOrList(repo *models.Repository, treePath, ref string) (interface{}, error) {
	if ref == "" {
		ref = repo.DefaultBranch
	}
	origRef := ref

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, models.ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, err
	}

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	if entry.Type() != "tree" {
		return GetFileContents(repo, treePath, origRef, false)
	}

	// We are in a directory, so we return a list of FileContentResponse objects
	var fileList []*api.FileContentsResponse

	gitTree, err := commit.SubTree(treePath)
	if err != nil {
		return nil, err
	}
	entries, err := gitTree.ListEntries()
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		subTreePath := path.Join(treePath, e.Name())
		fileContentResponse, err := GetFileContents(repo, subTreePath, origRef, true)
		if err != nil {
			return nil, err
		}
		fileList = append(fileList, fileContentResponse)
	}
	return fileList, nil
}

// GetFileContents gets the meta data on a file's contents. Ref can be a branch, commit or tag
func GetFileContents(repo *models.Repository, treePath, ref string, forList bool) (*api.FileContentsResponse, error) {
	if ref == "" {
		ref = repo.DefaultBranch
	}
	origRef := ref

	// Check that the path given in opts.treePath is valid (not a git path)
	cleanTreePath := CleanUploadFileName(treePath)
	if cleanTreePath == "" && treePath != "" {
		return nil, models.ErrFilenameInvalid{
			Path: treePath,
		}
	}
	treePath = cleanTreePath

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}

	// Get the commit object for the ref
	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, err
	}
	commitID := commit.ID.String()
	if len(ref) >= 4 && strings.HasPrefix(commitID, ref) {
		ref = commit.ID.String()
	}

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	refType := gitRepo.GetRefType(ref)
	if refType == "invalid" {
		return nil, fmt.Errorf("no commit found for the ref [ref: %s]", ref)
	}

	selfURL, _ := url.Parse(fmt.Sprintf("%s/contents/%s?ref=%s", repo.APIURL(), treePath, origRef))
	gitURL, _ := url.Parse(fmt.Sprintf("%s/git/blobs/%s", repo.APIURL(), entry.ID.String()))
	downloadURL, _ := url.Parse(fmt.Sprintf("%s/raw/%s/%s/%s", repo.HTMLURL(), refType, ref, treePath))
	htmlURL, _ := url.Parse(fmt.Sprintf("%s/src/%s/%s/%s", repo.HTMLURL(), refType, ref, treePath))

	contentType := ContentType("")
	target := ""
	content := ""
	encoding := ""
	submoduleURL := ""
	if entry.IsRegular() {
		contentType = ContentTypeRegular
		if blobResponse, err := GetBlobBySHA(repo, entry.ID.String()); err != nil {
			return nil, err
		} else if !forList {
			// We don't show the content if we are getting a list of FileContentResponses
			encoding = blobResponse.Encoding
			content = blobResponse.Content
		}
	} else if entry.IsDir() {
		contentType = ContentTypeDir
		downloadURL, _ = url.Parse("") // no download URL for dirs
	} else if entry.IsLink() {
		contentType = ContentTypeLink
		// The target of a symlink file is the content of the file
		targetFromContent, err := entry.Blob().GetBlobContent()
		if err != nil {
			return nil, err
		}
		target = targetFromContent
	} else if entry.IsSubModule() {
		contentType = ContentTypeSubmodule
		submodule, err := commit.GetSubModule(treePath)
		if err != nil {
			return nil, err
		}
		submoduleURL = submodule.URL
		htmlURL, _ = url.Parse("")     // TODO: if submodule is local, link to its trees HTML page
		gitURL, _ = url.Parse("")      // TODO: if submodule is local, link to its trees API endpoint
		downloadURL, _ = url.Parse("") // no download URL for submodules
	}

	fileContent := &api.FileContentsResponse{
		Name:            entry.Name(),
		Path:            treePath,
		Type:            contentType.String(),
		SHA:             entry.ID.String(),
		Size:            entry.Size(),
		Encoding:        encoding,
		Content:         content,
		Target:          target,
		URL:             selfURL.String(),
		HTMLURL:         htmlURL.String(),
		GitURL:          gitURL.String(),
		DownloadURL:     downloadURL.String(),
		SubmoduleGitURL: submoduleURL,
		Links: &api.FileLinksResponse{
			Self:    selfURL.String(),
			GitURL:  gitURL.String(),
			HTMLURL: htmlURL.String(),
		},
	}

	return fileContent, nil
}

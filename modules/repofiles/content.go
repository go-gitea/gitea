// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

// GetFileContents gets the meta data on a file's contents. Ref can be a branch, commit or tag
func GetFileContents(repo *models.Repository, treePath, ref string) (*api.FileContentResponse, error) {
	if ref == "" {
		ref = repo.DefaultBranch
	}
	origRef := ref

	// Check that the path given in opts.treePath is valid (not a git path)
	treePath = CleanUploadFileName(treePath)
	if treePath == "" {
		return nil, models.ErrFilenameInvalid{
			Path: treePath,
		}
	}

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

	blobResponse, err := GetBlobBySHA(repo, entry.ID.String())
	if err != nil {
		return nil, err
	}

	refType := gitRepo.GetRefType(ref)
	if refType == git.RepoRefTypeInvalid {
		return nil, fmt.Errorf("no commit found for the ref [ref: %s]", ref)
	}

	selfURL, _ := url.Parse(fmt.Sprintf("%s/contents/%s?ref=%s", repo.APIURL(), treePath, origRef))
	gitURL, _ := url.Parse(fmt.Sprintf("%s/git/blobs/%s", repo.APIURL(), entry.ID.String()))
	downloadURL, _ := url.Parse(fmt.Sprintf("%s/raw/%s/%s/%s", repo.HTMLURL(), refType.String(), ref, treePath))
	htmlURL, _ := url.Parse(fmt.Sprintf("%s/src/%s/%s/%s", repo.HTMLURL(), refType.String(), ref, treePath))

	fileContent := &api.FileContentResponse{
		Name:        entry.Name(),
		Path:        treePath,
		SHA:         entry.ID.String(),
		Size:        entry.Size(),
		URL:         selfURL.String(),
		HTMLURL:     htmlURL.String(),
		GitURL:      gitURL.String(),
		DownloadURL: downloadURL.String(),
		Type:        entry.Type(),
		Encoding:    blobResponse.Encoding,
		Content:     blobResponse.Content,
		Links: &api.FileLinksResponse{
			Self:    selfURL.String(),
			GitURL:  gitURL.String(),
			HTMLURL: htmlURL.String(),
		},
	}

	return fileContent, nil
}

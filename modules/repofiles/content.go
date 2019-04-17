// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"net/url"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/sdk/gitea"
)

// GetFileContents gets the meta data on a file's contents
func GetFileContents(repo *models.Repository, treePath, ref string) (*api.FileContentResponse, error) {
	if ref == "" {
		ref = repo.DefaultBranch
	}

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

	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}

	urlRef := ref
	if _, err := gitRepo.GetBranchCommit(ref); err == nil {
		urlRef = "branch/" + ref
	}

	selfURL, _ := url.Parse(repo.APIURL() + "/contents/" + treePath)
	gitURL, _ := url.Parse(repo.APIURL() + "/git/blobs/" + entry.ID.String())
	downloadURL, _ := url.Parse(repo.HTMLURL() + "/raw/" + urlRef + "/" + treePath)
	htmlURL, _ := url.Parse(repo.HTMLURL() + "/blob/" + ref + "/" + treePath)

	fileContent := &api.FileContentResponse{
		Name:        entry.Name(),
		Path:        treePath,
		SHA:         entry.ID.String(),
		Size:        entry.Size(),
		URL:         selfURL.String(),
		HTMLURL:     htmlURL.String(),
		GitURL:      gitURL.String(),
		DownloadURL: downloadURL.String(),
		Type:        string(entry.Type),
		Links: &api.FileLinksResponse{
			Self:    selfURL.String(),
			GitURL:  gitURL.String(),
			HTMLURL: htmlURL.String(),
		},
	}

	return fileContent, nil
}

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package file_handling

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/sdk/gitea"
	"net/url"
)

// GetFileContents gets the meta data on a file's contents
func GetFileContents(repo *models.Repository, treePath, ref string) (*gitea.FileContentResponse, error) {
	if ref == "" {
		ref = "master"
	}
	if repo == nil {
		return nil, Err	}

	// Check that the path given in opts.treePath is valid (not a git path)
	treePath = cleanUploadFileName(treePath)
	if treePath == "" {
		return nil, models.ErrFilenameInvalid{treePath}
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
	parents := make([]gitea.CommitMeta, commit.ParentCount())
	for i := 0; i <= commit.ParentCount(); i++ {
		if parent, err := commit.Parent(i); err == nil && parent != nil {
			parentCommitURL, _ := url.Parse(repo.APIURL() + "/git/commits/" + parent.ID.String())
			parents[i] = gitea.CommitMeta{
				SHA: parent.ID.String(),
				URL: parentCommitURL.String(),
			}
		}
	}

	htmlURL, _ := url.Parse(repo.HTMLURL() + "/blob/" + ref + "/" + treePath)

	fileContent := &gitea.FileContentResponse{
		Name:        entry.Name(),
		Path:        treePath,
		SHA:         entry.ID.String(),
		Size:        entry.Size(),
		URL:         selfURL.String(),
		HTMLURL:     htmlURL.String(),
		GitURL:      gitURL.String(),
		DownloadURL: downloadURL.String(),
		Type:        string(entry.Type),
		Links: &gitea.FileLinksResponse{
			Self:    selfURL.String(),
			GitURL:  gitURL.String(),
			HTMLURL: htmlURL.String(),
		},
	}

	return fileContent, nil
}

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdata

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"
)

// GetBlobBySHA get the BlobResponse of a repository using a sha hash.
func GetBlobBySHA(repo *models.Repository, sha string) (*api.BlobResponse, error) {
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	gitBlob, err := gitRepo.GetBlob(sha)
	if err != nil {
		return nil, err
	}
	br := &api.BlobResponse{
		SHA:      gitBlob.ID.String(),
		URL:      repo.APIURL() + "/git/blobs/" + gitBlob.ID.String(),
		Size:     gitBlob.Size(),
		Encoding: "base64",
	}
	if gitBlob.Size() <= setting.API.DefaultMaxBlobSize {
		br.Content = &api.BlobContentResponse{Blob: gitBlob}
	}
	return br, nil
}

// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdata

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
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
		Content: &api.BlobContentResponse{
			Blob: gitBlob,
		},
		SHA:      gitBlob.ID.String(),
		URL:      repo.APIURL() + "/git/blobs/" + gitBlob.ID.String(),
		Size:     gitBlob.Size(),
		Encoding: "base64",
	}
	return br, nil
}

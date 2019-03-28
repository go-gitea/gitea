// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdata

import (
	"encoding/base64"
	"io"
	"io/ioutil"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/sdk/gitea"
)

// GetBlobBySHA get the GitBlobResponse of a repository using a sha hash.
func GetBlobBySHA(repo *models.Repository, sha string) (*api.GitBlobResponse, error) {
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return nil, err
	}
	gitBlob, err := gitRepo.GetBlob(sha)
	if err != nil {
		return nil, err
	}
	blob := new(api.GitBlobResponse)
	blob.SHA = gitBlob.ID.String()
	blob.URL = repo.APIURL() + "/git/blobs/" + blob.SHA
	blob.Size = gitBlob.Size()
	blob.Encoding = "base64"
	blob.Content, err = GetBlobContentBase64(gitBlob)
	if err != nil {
		return nil, err
	}
	return blob, nil
}

// GetBlobContentBase64 Reads the blob with a base64 encode and returns the encoded string
func GetBlobContentBase64(blob *git.Blob) (string, error) {
	dataRc, err := blob.DataAsync()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()

	pr, pw := io.Pipe()
	encoder := base64.NewEncoder(base64.StdEncoding, pw)

	go func() {
		_, err := io.Copy(encoder, dataRc)
		encoder.Close()

		if err != nil {
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}()

	out, err := ioutil.ReadAll(pr)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

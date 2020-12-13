// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tests

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
	"github.com/stretchr/testify/assert"
)

func TestRepository_GetBlob_Found(t *testing.T) {
	RunTestPerProvider(t, func(service service.GitService, t *testing.T) {
		repoPath := filepath.Join(testReposDir, "repo1_bare")
		r, err := service.OpenRepository(repoPath)
		assert.NoError(t, err)
		defer r.Close()

		testCases := []struct {
			OID  string
			Data []byte
		}{
			{"e2129701f1a4d54dc44f03c93bca0a2aec7c5449", []byte("file1\n")},
			{"6c493ff740f9380390d5c9ddef4af18697ac9375", []byte("file2\n")},
		}

		for _, testCase := range testCases {
			blob, err := r.GetBlob(testCase.OID)
			assert.NoError(t, err)

			dataReader, err := blob.Reader()
			assert.NoError(t, err)
			defer dataReader.Close()

			data, err := ioutil.ReadAll(dataReader)
			assert.NoError(t, err)
			assert.Equal(t, testCase.Data, data)
		}
	})
}

func TestRepository_GetBlob_NotExist(t *testing.T) {
	RunTestPerProvider(t, func(service service.GitService, t *testing.T) {
		repoPath := filepath.Join(testReposDir, "repo1_bare")
		r, err := service.OpenRepository(repoPath)
		assert.NoError(t, err)
		defer r.Close()

		testCase := "0000000000000000000000000000000000000000"
		testError := git.ErrNotExist{ID: testCase, RelPath: ""}

		blob, err := r.GetBlob(testCase)
		assert.Nil(t, blob)
		assert.EqualError(t, err, testError.Error())
	})
}

func TestRepository_GetBlob_NoId(t *testing.T) {
	RunTestPerProvider(t, func(service service.GitService, t *testing.T) {
		repoPath := filepath.Join(testReposDir, "repo1_bare")
		r, err := service.OpenRepository(repoPath)
		assert.NoError(t, err)
		defer r.Close()

		testCase := ""
		testError := fmt.Errorf("Length must be 40: %s", testCase)

		blob, err := r.GetBlob(testCase)
		assert.Nil(t, blob)
		assert.EqualError(t, err, testError.Error())
	})
}

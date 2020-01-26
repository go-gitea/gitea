// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFormatPatch(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	clonedPath, err := cloneRepo(bareRepo1Path, testReposDir, "repo1_TestGetFormatPatch")
	assert.NoError(t, err)
	defer os.RemoveAll(clonedPath)
	repo, err := OpenRepository(clonedPath)
	assert.NoError(t, err)
	defer repo.Close()
	rd := &bytes.Buffer{}
	err = repo.GetPatch("8d92fc95^", "8d92fc95", rd)
	assert.NoError(t, err)
	patchb, err := ioutil.ReadAll(rd)
	assert.NoError(t, err)
	patch := string(patchb)
	assert.Regexp(t, "^From 8d92fc95", patch)
	assert.Contains(t, patch, "Subject: [PATCH] Add file2.txt")
}

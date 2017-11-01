// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/git"

	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
)

func TestClonePush_ViaHTTP_NoLogin(t *testing.T) {
	prepareTestEnv(t)

	s := http.Server{
		Addr:    ":3000",
		Handler: mac,
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.ListenAndServe()

	dstPath, err := ioutil.TempDir("", "repo1")
	assert.NoError(t, err)
	defer os.RemoveAll(dstPath)

	err := git.Clone("http://localhost:3000/user2/repo1.git", dstPath, git.CloneRepoOptions{})
	assert.NoError(t, err)

	assert.True(t, com.IsExist(filepath.Join(dstPath, "README.md")))
}

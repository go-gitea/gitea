// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
	"github.com/stretchr/testify/assert"
)

func assertFileExist(t *testing.T, p string) {
	exist, err := util.IsExist(p)
	assert.NoError(t, err)
	assert.True(t, exist)
}

func assertFileEqual(t *testing.T, p string, content []byte) {
	bs, err := os.ReadFile(p)
	assert.NoError(t, err)
	assert.EqualValues(t, content, bs)
}

func TestRepoCloneWiki(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		defer prepareTestEnv(t)()

		dstPath, err := os.MkdirTemp("", "clone_wiki")
		assert.NoError(t, err)

		r := fmt.Sprintf("%suser2/repo1.wiki.git", u.String())
		u, _ = url.Parse(r)
		u.User = url.UserPassword("user2", userPassword)
		t.Run("Clone", func(t *testing.T) {
			assert.NoError(t, git.CloneWithArgs(context.Background(), u.String(), dstPath, allowLFSFilters(), git.CloneRepoOptions{}))
			assertFileEqual(t, filepath.Join(dstPath, "Home.md"), []byte("# Home page\n\nThis is the home page!\n"))
			assertFileExist(t, filepath.Join(dstPath, "Page-With-Image.md"))
			assertFileExist(t, filepath.Join(dstPath, "Page-With-Spaced-Name.md"))
			assertFileExist(t, filepath.Join(dstPath, "images"))
			assertFileExist(t, filepath.Join(dstPath, "jpeg.jpg"))
		})
	})
}

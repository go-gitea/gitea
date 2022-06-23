// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/lfs"

	"github.com/stretchr/testify/assert"
)

func str2url(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

func TestDetermineLocalEndpoint(t *testing.T) {
	defer prepareTestEnv(t)()

	root, _ := os.MkdirTemp("", "lfs_test")
	defer os.RemoveAll(root)

	rootdotgit, _ := os.MkdirTemp("", "lfs_test")
	defer os.RemoveAll(rootdotgit)
	os.Mkdir(filepath.Join(rootdotgit, ".git"), 0o700)

	lfsroot, _ := os.MkdirTemp("", "lfs_test")
	defer os.RemoveAll(lfsroot)

	// Test cases
	cases := []struct {
		cloneurl string
		lfsurl   string
		expected *url.URL
	}{
		// case 0
		{
			cloneurl: root,
			lfsurl:   "",
			expected: str2url(fmt.Sprintf("file://%s", root)),
		},
		// case 1
		{
			cloneurl: root,
			lfsurl:   lfsroot,
			expected: str2url(fmt.Sprintf("file://%s", lfsroot)),
		},
		// case 2
		{
			cloneurl: "https://git.com/repo.git",
			lfsurl:   lfsroot,
			expected: str2url(fmt.Sprintf("file://%s", lfsroot)),
		},
		// case 3
		{
			cloneurl: rootdotgit,
			lfsurl:   "",
			expected: str2url(fmt.Sprintf("file://%s", filepath.Join(rootdotgit, ".git"))),
		},
		// case 4
		{
			cloneurl: "",
			lfsurl:   rootdotgit,
			expected: str2url(fmt.Sprintf("file://%s", filepath.Join(rootdotgit, ".git"))),
		},
		// case 5
		{
			cloneurl: rootdotgit,
			lfsurl:   rootdotgit,
			expected: str2url(fmt.Sprintf("file://%s", filepath.Join(rootdotgit, ".git"))),
		},
		// case 6
		{
			cloneurl: fmt.Sprintf("file://%s", root),
			lfsurl:   "",
			expected: str2url(fmt.Sprintf("file://%s", root)),
		},
		// case 7
		{
			cloneurl: fmt.Sprintf("file://%s", root),
			lfsurl:   fmt.Sprintf("file://%s", lfsroot),
			expected: str2url(fmt.Sprintf("file://%s", lfsroot)),
		},
		// case 8
		{
			cloneurl: root,
			lfsurl:   fmt.Sprintf("file://%s", lfsroot),
			expected: str2url(fmt.Sprintf("file://%s", lfsroot)),
		},
		// case 9
		{
			cloneurl: "",
			lfsurl:   "/does/not/exist",
			expected: nil,
		},
		// case 10
		{
			cloneurl: "",
			lfsurl:   "file:///does/not/exist",
			expected: str2url("file:///does/not/exist"),
		},
	}

	for n, c := range cases {
		ep := lfs.DetermineEndpoint(c.cloneurl, c.lfsurl)

		assert.Equal(t, c.expected, ep, "case %d: error should match", n)
	}
}

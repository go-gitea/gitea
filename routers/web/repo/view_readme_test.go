// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"path"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindReadmeFileInEntriesWithSymlinkInSubfolder(t *testing.T) {
	for _, subdir := range []string{".github", ".gitea", "docs"} {
		t.Run(subdir, func(t *testing.T) {
			repoPath := t.TempDir()
			stdin := fmt.Sprintf(`commit refs/heads/master
author Test <test@example.com> 1700000000 +0000
committer Test <test@example.com> 1700000000 +0000
data <<EOT
initial
EOT
M 100644 inline target.md
data <<EOT
target-content
EOT
M 120000 inline %s/README.md
data 12
../target.md
`, subdir)

			var err error
			err = gitcmd.NewCommand("init", "--bare", ".").WithDir(repoPath).RunWithStderr(t.Context())
			require.NoError(t, err)
			err = gitcmd.NewCommand("fast-import").WithDir(repoPath).WithStdinBytes([]byte(stdin)).RunWithStderr(t.Context())
			require.NoError(t, err)

			gitRepo, err := git.OpenRepository(t.Context(), repoPath)
			require.NoError(t, err)
			defer gitRepo.Close()

			commit, err := gitRepo.GetBranchCommit("master")
			require.NoError(t, err)

			entries, err := commit.ListEntries()
			require.NoError(t, err)

			ctx, _ := contexttest.MockContext(t, "/")
			ctx.Repo.Commit = commit
			ctx.Repo.TreePath = ""
			subfolder, readmeFile, err := findReadmeFileInEntries(ctx, "", entries, true)
			require.NoError(t, err)
			require.NotNil(t, readmeFile)

			assert.Equal(t, subdir, subfolder)
			assert.Equal(t, "README.md", readmeFile.Name())
			assert.True(t, readmeFile.IsLink())

			// Verify that it can follow the link
			res, err := git.EntryFollowLinks(commit, path.Join(subfolder, readmeFile.Name()), readmeFile)
			require.NoError(t, err)
			assert.Equal(t, "target.md", res.TargetFullPath)
		})
	}
}

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"os"
	"path"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindReadmeFileInEntriesWithSymlinkInSubfolder(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// Create a target README file content
	targetContent := "This is the target README."

	subdirs := []string{".github", ".gitea", "docs"}

	for _, subdir := range subdirs {
		t.Run(subdir, func(t *testing.T) {
			// Create a temporary git repository for each case to avoid interference
			repoPath := t.TempDir()
			require.NoError(t, git.InitRepository(context.Background(), repoPath, false, git.Sha1ObjectFormat.Name()))
			repo, err := git.OpenRepository(context.Background(), repoPath)
			require.NoError(t, err)
			defer repo.Close()

			// Create a target README file
			targetFile := "target.md"
			require.NoError(t, os.WriteFile(path.Join(repoPath, targetFile), []byte(targetContent), 0o644))

			// Create a symlinked README.md in the subfolder
			dirPath := path.Join(repoPath, subdir)
			require.NoError(t, os.Mkdir(dirPath, 0o755))
			require.NoError(t, os.Symlink("../target.md", path.Join(dirPath, "README.md")))

			// Commit the files
			require.NoError(t, git.AddChanges(context.Background(), repoPath, true))
			require.NoError(t, git.CommitChanges(context.Background(), repoPath, git.CommitChangesOptions{
				Message: "Add symlinked README in " + subdir,
				Author: &git.Signature{
					Name:  "Test",
					Email: "test@example.com",
				},
			}))

			commit, err := repo.GetBranchCommit("master")
			require.NoError(t, err)

			entries, err := commit.ListEntries()
			require.NoError(t, err)

			ctx, _ := contexttest.MockContext(t, "/")
			ctx.Repo.Commit = commit
			ctx.Repo.TreePath = ""

			subfolder, readmeFile, err := findReadmeFileInEntries(ctx, "", entries, true)
			require.NoError(t, err)

			assert.Equal(t, subdir, subfolder)
			require.NotNil(t, readmeFile)
			assert.Equal(t, "README.md", readmeFile.Name())
			assert.True(t, readmeFile.IsLink())

			// Verify that it can follow the link
			res, err := git.EntryFollowLinks(commit, path.Join(subfolder, readmeFile.Name()), readmeFile)
			require.NoError(t, err)
			assert.Equal(t, "target.md", res.TargetFullPath)
		})
	}
}

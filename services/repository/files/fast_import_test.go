// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestFastImport(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// Initialize the repository
	repoPath, err := os.MkdirTemp("", "test-repo-*.git")
	assert.NoError(t, err)
	defer os.RemoveAll(repoPath)

	_, _, err = git.NewCommand("init", "--bare").RunStdString(t.Context(),
		&git.RunOpts{
			Dir: repoPath,
		})
	assert.NoError(t, err)

	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "fast_import_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilePath := fmt.Sprintf("%s/testfile.txt", tempDir)
	err = os.WriteFile(testFilePath, []byte("Hello, World!"), 0o644)
	assert.NoError(t, err)

	f, err := os.Open(testFilePath)
	assert.NoError(t, err)
	defer f.Close()

	doer, err := user_model.GetUserByID(t.Context(), 1)
	assert.NoError(t, err)

	// 1 - Create the commit file
	t.Run("Create commit file", func(t *testing.T) {
		// Prepare the ChangeRepoFilesOptions
		options := ChangeRepoFilesOptions{
			LastCommitID: "HEAD",
			NewBranch:    "test-branch",
			Message:      "Test commit",
			Files: []*ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "testfile.txt",
					ContentReader: f,
				},
			},
			Author: &IdentityOptions{
				GitUserName:  "Test User",
				GitUserEmail: "testuser@gitea.com",
			},
			Committer: &IdentityOptions{
				GitUserName:  "Test Committer",
				GitUserEmail: "testuser@gitea.com",
			},
			Dates: &CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		}

		err = UpdateRepoBranch(t.Context(), doer, repoPath, options)
		assert.NoError(t, err)

		gitRepo, err := git.OpenRepository(t.Context(), repoPath)
		assert.NoError(t, err)
		defer gitRepo.Close()

		branches, total, err := gitRepo.GetBranchNames(0, 0)
		assert.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Equal(t, 1, len(branches))
		assert.Equal(t, "test-branch", branches[0])

		commit, err := gitRepo.GetBranchCommit("test-branch")
		assert.NoError(t, err)
		assert.Equal(t, "Test commit\n", commit.Message())
		entries, err := commit.Tree.ListEntries()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(entries))
		assert.Equal(t, "testfile.txt", entries[0].Name())
		content, err := entries[0].Blob().GetBlobContent(entries[0].Blob().Size())
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World!\n", content)
	})

	// 2 - Add a new file and update the existing one in a new branch
	t.Run("Add and update files", func(t *testing.T) {
		f2 := bytes.NewReader([]byte("Hello, World! 1"))
		_, err = f.Seek(0, io.SeekStart)
		assert.NoError(t, err)
		options := ChangeRepoFilesOptions{
			LastCommitID: "HEAD",
			OldBranch:    "test-branch",
			NewBranch:    "test-branch-2",
			Message:      "Test commit-2",
			Files: []*ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "testfile2.txt",
					ContentReader: f,
				},
				{
					Operation:     "update",
					TreePath:      "testfile.txt",
					ContentReader: f2,
				},
			},
			Author: &IdentityOptions{
				GitUserName:  "Test User",
				GitUserEmail: "testuser@gitea.com",
			},
			Committer: &IdentityOptions{
				GitUserName:  "Test Committer",
				GitUserEmail: "testuser@gitea.com",
			},
			Dates: &CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		}
		err = UpdateRepoBranch(t.Context(), doer, repoPath, options)
		assert.NoError(t, err)

		gitRepo, err := git.OpenRepository(t.Context(), repoPath)
		assert.NoError(t, err)
		defer gitRepo.Close()

		branches, total, err := gitRepo.GetBranchNames(0, 0)
		assert.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Equal(t, 2, len(branches))
		assert.Equal(t, "test-branch", branches[0])
		assert.Equal(t, "test-branch-2", branches[1])

		commit, err := gitRepo.GetBranchCommit("test-branch-2")
		assert.NoError(t, err)
		assert.Equal(t, "Test commit-2\n", commit.Message())
		entries, err := commit.Tree.ListEntries()
		assert.NoError(t, err)
		assert.Equal(t, 2, len(entries))
		assert.Equal(t, "testfile.txt", entries[0].Name())
		assert.Equal(t, "testfile2.txt", entries[1].Name())
		content, err := entries[0].Blob().GetBlobContent(entries[0].Blob().Size())
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World! 1\n", content)

		content, err = entries[1].Blob().GetBlobContent(entries[1].Blob().Size())
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World!\n", content)
	})

	// 3 - Delete the file
	t.Run("Delete file in the same branch", func(t *testing.T) {
		options := ChangeRepoFilesOptions{
			OldBranch: "test-branch-2",
			Message:   "Test commit-3",
			Files: []*ChangeRepoFile{
				{
					Operation:    "delete",
					FromTreePath: "testfile.txt",
				},
			},
			Author: &IdentityOptions{
				GitUserName:  "Test User",
				GitUserEmail: "testuser@gitea.com",
			},
			Committer: &IdentityOptions{
				GitUserName:  "Test Committer",
				GitUserEmail: "testuser@gitea.com",
			},
			Dates: &CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		}
		err = UpdateRepoBranch(t.Context(), doer, repoPath, options)
		assert.NoError(t, err)

		gitRepo, err := git.OpenRepository(t.Context(), repoPath)
		assert.NoError(t, err)
		defer gitRepo.Close()

		branches, total, err := gitRepo.GetBranchNames(0, 0)
		assert.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Equal(t, 2, len(branches))
		assert.Equal(t, "test-branch", branches[0])
		assert.Equal(t, "test-branch-2", branches[1])

		commit, err := gitRepo.GetBranchCommit("test-branch-2")
		assert.NoError(t, err)
		assert.Equal(t, "Test commit-3\n", commit.Message())
		entries, err := commit.Tree.ListEntries()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(entries))
		assert.Equal(t, "testfile2.txt", entries[0].Name())
		content, err := entries[0].Blob().GetBlobContent(entries[0].Blob().Size())
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World!\n", content)
	})

	// 4 - Delete the file in a new branch
	t.Run("Delete file in a new branch", func(t *testing.T) {
		options := ChangeRepoFilesOptions{
			OldBranch: "test-branch-2",
			NewBranch: "test-branch-3",
			Message:   "Test commit-4",
			Files: []*ChangeRepoFile{
				{
					Operation:    "delete",
					FromTreePath: "testfile2.txt",
				},
			},
			Author: &IdentityOptions{
				GitUserName:  "Test User",
				GitUserEmail: "testuser@gitea.com",
			},
			Committer: &IdentityOptions{
				GitUserName:  "Test Committer",
				GitUserEmail: "testuser@gitea.com",
			},
			Dates: &CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		}
		err = UpdateRepoBranch(t.Context(), doer, repoPath, options)
		assert.NoError(t, err)

		gitRepo, err := git.OpenRepository(t.Context(), repoPath)
		assert.NoError(t, err)
		defer gitRepo.Close()

		branches, total, err := gitRepo.GetBranchNames(0, 0)
		assert.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Equal(t, 3, len(branches))
		assert.True(t, slices.Equal(branches, []string{"test-branch", "test-branch-2", "test-branch-3"}))

		commit, err := gitRepo.GetBranchCommit("test-branch-3")
		assert.NoError(t, err)
		assert.Equal(t, "Test commit-4\n", commit.Message())
		entries, err := commit.Tree.ListEntries()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(entries))
	})

	// 5 - add/delete the file in a new branch from test-branch-2
	t.Run("Add/Delete file in a new branch", func(t *testing.T) {
		options := ChangeRepoFilesOptions{
			OldBranch: "test-branch-2",
			NewBranch: "test-branch-4",
			Message:   "Test commit-5",
			Files: []*ChangeRepoFile{
				{
					Operation:     "create",
					TreePath:      "testfile3.txt",
					ContentReader: bytes.NewReader([]byte("Hello, World! 3")),
				},
				{
					Operation:    "delete",
					FromTreePath: "testfile2.txt",
				},
			},
			Author: &IdentityOptions{
				GitUserName:  "Test User",
				GitUserEmail: "testuser@gitea.com",
			},
			Committer: &IdentityOptions{
				GitUserName:  "Test Committer",
				GitUserEmail: "testuser@gitea.com",
			},
			Dates: &CommitDateOptions{
				Author:    time.Now(),
				Committer: time.Now(),
			},
		}
		err = UpdateRepoBranch(t.Context(), doer, repoPath, options)
		assert.NoError(t, err)

		gitRepo, err := git.OpenRepository(t.Context(), repoPath)
		assert.NoError(t, err)
		defer gitRepo.Close()

		branches, total, err := gitRepo.GetBranchNames(0, 0)
		assert.NoError(t, err)
		assert.Equal(t, 4, total)
		assert.Equal(t, 4, len(branches))
		assert.True(t, slices.Equal(branches, []string{"test-branch", "test-branch-2", "test-branch-3", "test-branch-4"}))

		commit, err := gitRepo.GetBranchCommit("test-branch-4")
		assert.NoError(t, err)
		assert.Equal(t, "Test commit-5\n", commit.Message())
		entries, err := commit.Tree.ListEntries()
		assert.NoError(t, err)
		assert.Equal(t, 1, len(entries))
		assert.Equal(t, "testfile3.txt", entries[0].Name())
		content, err := entries[0].Blob().GetBlobContent(entries[0].Blob().Size())
		assert.NoError(t, err)
		assert.Equal(t, "Hello, World! 3\n", content)
	})
}

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build zoekt && unix

package zoekt

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code/internal"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRepo struct {
	*repo_model.Repository
	git func(args ...string) string
}

// newTestRepo creates an empty git repository below setting.RepoRootPath
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()

	repo := &repo_model.Repository{ID: 1, OwnerName: "owner", Name: "repo"}
	repoPath := filepath.Join(setting.RepoRootPath, filepath.FromSlash(repo.GitRepoLocation()))
	require.NoError(t, os.MkdirAll(repoPath, 0o755))

	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=gitea", "GIT_AUTHOR_EMAIL=gitea@example.com",
			"GIT_COMMITTER_NAME=gitea", "GIT_COMMITTER_EMAIL=gitea@example.com",
			// keep the developer's own git config out of the fixture
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	return &testRepo{Repository: repo, git: git}
}

// commit writes the given files and commits them, returning the new HEAD SHA
func (r *testRepo) commit(t *testing.T, files map[string][]byte) string {
	t.Helper()
	repoPath := filepath.Join(setting.RepoRootPath, filepath.FromSlash(r.GitRepoLocation()))
	for name, content := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(repoPath, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repoPath, name), content, 0o644))
	}
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "commit")
	return r.git("rev-parse", "HEAD")
}

// update builds the changes for the given files as the code indexer would for a
// push, so only the changed files are handed to the indexer
func (r *testRepo) update(t *testing.T, filenames ...string) *internal.RepoChanges {
	t.Helper()
	stdout := r.git(append([]string{"ls-tree", "--full-tree", "-l", "HEAD", "--"}, filenames...)...)
	gitRepo, err := git.OpenRepository(t.Context(), r.Repository)
	require.NoError(t, err)
	defer gitRepo.Close()
	updates, err := internal.ParseGitLsTreeOutput(t.Context(), gitRepo, []byte(stdout+"\n"))
	require.NoError(t, err)
	require.Len(t, updates, len(filenames))
	return &internal.RepoChanges{Updates: updates}
}

// searchFilenames waits until the searcher picked up the index and the keyword
// matches the wanted files, the directory watcher does not reload synchronously
func searchFilenames(t *testing.T, idx *Indexer, keyword string, want []string) {
	t.Helper()
	var files []string
	assert.Eventually(t, func() bool {
		_, res, _, err := idx.Search(t.Context(), &internal.SearchOptions{
			Keyword:    keyword,
			SearchMode: indexer.SearchModeExact,
		})
		require.NoError(t, err)
		files = files[:0]
		for _, r := range res {
			files = append(files, r.Filename)
		}
		slices.Sort(files)
		return slices.Equal(files, slices.Sorted(slices.Values(want)))
	}, 10*time.Second, 100*time.Millisecond, "searching %q: want %v, got %v", keyword, want, files)
}

// A binary blob has no content to index, but it shares the cat-file batch reader
// with the files that follow it, so skipping it must not affect their content.
func TestIndexRepoWithBinaryFile(t *testing.T) {
	defer test.MockVariableValue(&setting.RepoRootPath, t.TempDir())()
	defer test.MockVariableValue(&setting.Indexer.MaxIndexerFileSize, int64(1<<20))()

	repo := newTestRepo(t)
	sha := repo.commit(t, map[string][]byte{
		// git ls-tree lists the files in name order, so the binary one comes first
		"a.bin": append([]byte("\x89PNG\r\n\x1a\n"), 0, 1, 2, 3),
		"b.txt": []byte("needle in b\n"),
		"c.txt": []byte("needle in c\n"),
	})

	idx := NewIndexer(t.TempDir())
	defer idx.Close()
	_, err := idx.Init(t.Context())
	require.NoError(t, err)

	gitRepo, err := git.OpenRepository(t.Context(), repo.Repository)
	require.NoError(t, err)
	defer gitRepo.Close()
	changes, err := internal.GenesisChanges(t.Context(), repo.Repository, gitRepo, sha)
	require.NoError(t, err)
	require.Len(t, changes.Updates, 3)
	require.NoError(t, idx.Index(t.Context(), repo.Repository, sha, changes))

	searchFilenames(t, idx, "needle", []string{"b.txt", "c.txt"})
}

func TestIndexRepoIncrementally(t *testing.T) {
	defer test.MockVariableValue(&setting.RepoRootPath, t.TempDir())()
	defer test.MockVariableValue(&setting.Indexer.MaxIndexerFileSize, int64(1<<20))()

	repo := newTestRepo(t)
	sha := repo.commit(t, map[string][]byte{
		"a.txt": []byte("needle in a\n"),
		"b.txt": []byte("needle in b\n"),
	})

	idx := NewIndexer(t.TempDir())
	defer idx.Close()
	_, err := idx.Init(t.Context())
	require.NoError(t, err)

	gitRepo, err := git.OpenRepository(t.Context(), repo.Repository)
	require.NoError(t, err)
	defer gitRepo.Close()
	changes, err := internal.GenesisChanges(t.Context(), repo.Repository, gitRepo, sha)
	require.NoError(t, err)
	require.NoError(t, idx.Index(t.Context(), repo.Repository, sha, changes))
	searchFilenames(t, idx, "needle", []string{"a.txt", "b.txt"})

	t.Run("added and changed files", func(t *testing.T) {
		sha = repo.commit(t, map[string][]byte{
			"b.txt": []byte("haystack in b\n"),
			"c.txt": []byte("needle in c\n"),
		})
		require.NoError(t, idx.Index(t.Context(), repo.Repository, sha, repo.update(t, "b.txt", "c.txt")))

		searchFilenames(t, idx, "needle", []string{"a.txt", "c.txt"})
		searchFilenames(t, idx, "haystack", []string{"b.txt"})
	})

	t.Run("removed file", func(t *testing.T) {
		repo.git("rm", "-q", "a.txt")
		repo.git("commit", "-q", "-m", "remove")
		sha = repo.git("rev-parse", "HEAD")
		require.NoError(t, idx.Index(t.Context(), repo.Repository, sha, &internal.RepoChanges{
			RemovedFilenames: []string{"a.txt"},
		}))

		searchFilenames(t, idx, "needle", []string{"c.txt"})
	})

	t.Run("changed build options rebuild the whole index", func(t *testing.T) {
		// the file size limit is part of zoekt's option hash, changing it makes the
		// existing shards unusable for a delta build
		defer test.MockVariableValue(&setting.Indexer.MaxIndexerFileSize, int64(2<<20))()

		sha = repo.commit(t, map[string][]byte{"d.txt": []byte("needle in d\n")})
		require.NoError(t, idx.Index(t.Context(), repo.Repository, sha, repo.update(t, "d.txt")))

		// the files of the earlier commits are not part of the changes above, they
		// are only found again if the index was rebuilt from the full tree
		searchFilenames(t, idx, "needle", []string{"c.txt", "d.txt"})
		searchFilenames(t, idx, "haystack", []string{"b.txt"})
	})
}

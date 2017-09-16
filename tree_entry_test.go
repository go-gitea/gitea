// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const benchmarkReposDir = "benchmark/repos/"

func setupGitRepo(url string, name string) (string, error) {
	repoDir := filepath.Join(benchmarkReposDir, name)
	if _, err := os.Stat(repoDir); err == nil {
		return repoDir, nil
	}
	return repoDir, Clone(url, repoDir, CloneRepoOptions{
		Mirror:  false,
		Bare:    false,
		Quiet:   true,
		Timeout: 5 * time.Minute,
	})
}

func BenchmarkEntries_GetCommitsInfo(b *testing.B) {
	benchmarks := []struct {
		url  string
		name string
	}{
		{url: "https://github.com/go-gitea/gitea.git", name: "gitea"},
		{url: "https://github.com/ethantkoenig/manyfiles.git", name: "manyfiles"},
		{url: "https://github.com/moby/moby.git", name: "moby"},
		{url: "https://github.com/golang/go.git", name: "go"},
		{url: "https://github.com/torvalds/linux.git", name: "linux"},
	}
	for _, benchmark := range benchmarks {
		var commit *Commit
		var entries Entries
		if repoPath, err := setupGitRepo(benchmark.url, benchmark.name); err != nil {
			b.Fatal(err)
		} else if repo, err := OpenRepository(repoPath); err != nil {
			b.Fatal(err)
		} else if commit, err = repo.GetBranchCommit("master"); err != nil {
			b.Fatal(err)
		} else if entries, err = commit.Tree.ListEntries(); err != nil {
			b.Fatal(err)
		}
		entries.Sort()
		b.ResetTimer()
		b.Run(benchmark.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := entries.GetCommitsInfo(commit, "")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func getTestEntries() Entries {
	return Entries{
		&TreeEntry{name: "v1.0", mode: EntryModeTree},
		&TreeEntry{name: "v2.0", mode: EntryModeTree},
		&TreeEntry{name: "v2.1", mode: EntryModeTree},
		&TreeEntry{name: "v2.12", mode: EntryModeTree},
		&TreeEntry{name: "v2.2", mode: EntryModeTree},
		&TreeEntry{name: "v12.0", mode: EntryModeTree},
		&TreeEntry{name: "abc", mode: EntryModeBlob},
		&TreeEntry{name: "bcd", mode: EntryModeBlob},
	}
}

func TestEntriesSort(t *testing.T) {
	entries := getTestEntries()
	entries.Sort()
	assert.Equal(t, "v1.0", entries[0].Name())
	assert.Equal(t, "v12.0", entries[1].Name())
	assert.Equal(t, "v2.0", entries[2].Name())
	assert.Equal(t, "v2.1", entries[3].Name())
	assert.Equal(t, "v2.12", entries[4].Name())
	assert.Equal(t, "v2.2", entries[5].Name())
	assert.Equal(t, "abc", entries[6].Name())
	assert.Equal(t, "bcd", entries[7].Name())
}

func TestEntriesCustomSort(t *testing.T) {
	entries := getTestEntries()
	entries.CustomSort(func(s1, s2 string) bool {
		return s1 > s2
	})
	assert.Equal(t, "v2.2", entries[0].Name())
	assert.Equal(t, "v2.12", entries[1].Name())
	assert.Equal(t, "v2.1", entries[2].Name())
	assert.Equal(t, "v2.0", entries[3].Name())
	assert.Equal(t, "v12.0", entries[4].Name())
	assert.Equal(t, "v1.0", entries[5].Name())
	assert.Equal(t, "bcd", entries[6].Name())
	assert.Equal(t, "abc", entries[7].Name())
}

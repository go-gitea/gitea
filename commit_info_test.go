package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

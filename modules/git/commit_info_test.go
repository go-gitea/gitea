// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testReposDir = "tests/repos/"
)

func cloneRepo(tb testing.TB, url string) (string, error) {
	repoDir := tb.TempDir()
	if err := Clone(DefaultContext, url, repoDir, CloneRepoOptions{
		Mirror:  false,
		Bare:    false,
		Quiet:   true,
		Timeout: 5 * time.Minute,
	}); err != nil {
		return "", err
	}
	return repoDir, nil
}

func testGetCommitsInfo(t *testing.T, repo1 *Repository) {
	// these test case are specific to the repo1 test repo
	testCases := []struct {
		CommitID           string
		Path               string
		ExpectedIDs        map[string]string
		ExpectedTreeCommit string
	}{
		{"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2", "", map[string]string{
			"file1.txt": "95bb4d39648ee7e325106df01a621c530863a653",
			"file2.txt": "8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
		}, "8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2"},
		{"2839944139e0de9737a044f78b0e4b40d989a9e3", "", map[string]string{
			"file1.txt":   "2839944139e0de9737a044f78b0e4b40d989a9e3",
			"branch1.txt": "9c9aef8dd84e02bc7ec12641deb4c930a7c30185",
		}, "2839944139e0de9737a044f78b0e4b40d989a9e3"},
		{"5c80b0245c1c6f8343fa418ec374b13b5d4ee658", "branch2", map[string]string{
			"branch2.txt": "5c80b0245c1c6f8343fa418ec374b13b5d4ee658",
		}, "5c80b0245c1c6f8343fa418ec374b13b5d4ee658"},
		{"feaf4ba6bc635fec442f46ddd4512416ec43c2c2", "", map[string]string{
			"file1.txt": "95bb4d39648ee7e325106df01a621c530863a653",
			"file2.txt": "8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
			"foo":       "37991dec2c8e592043f47155ce4808d4580f9123",
		}, "feaf4ba6bc635fec442f46ddd4512416ec43c2c2"},
	}
	for _, testCase := range testCases {
		commit, err := repo1.GetCommit(testCase.CommitID)
		if err != nil {
			assert.NoError(t, err, "Unable to get commit: %s from testcase due to error: %v", testCase.CommitID, err)
			// no point trying to do anything else for this test.
			continue
		}
		assert.NotNil(t, commit)
		assert.NotNil(t, commit.Tree)
		assert.NotNil(t, commit.Tree.repo)

		tree, err := commit.Tree.SubTree(testCase.Path)
		if err != nil {
			assert.NoError(t, err, "Unable to get subtree: %s of commit: %s from testcase due to error: %v", testCase.Path, testCase.CommitID, err)
			// no point trying to do anything else for this test.
			continue
		}

		assert.NotNil(t, tree, "tree is nil for testCase CommitID %s in Path %s", testCase.CommitID, testCase.Path)
		assert.NotNil(t, tree.repo, "repo is nil for testCase CommitID %s in Path %s", testCase.CommitID, testCase.Path)

		entries, err := tree.ListEntries()
		if err != nil {
			assert.NoError(t, err, "Unable to get entries of subtree: %s in commit: %s from testcase due to error: %v", testCase.Path, testCase.CommitID, err)
			// no point trying to do anything else for this test.
			continue
		}

		// FIXME: Context.TODO() - if graceful has started we should use its Shutdown context otherwise use install signals in TestMain.
		commitsInfo, treeCommit, err := entries.GetCommitsInfo(t.Context(), commit, testCase.Path)
		assert.NoError(t, err, "Unable to get commit information for entries of subtree: %s in commit: %s from testcase due to error: %v", testCase.Path, testCase.CommitID, err)
		if err != nil {
			t.FailNow()
		}
		assert.Equal(t, testCase.ExpectedTreeCommit, treeCommit.ID.String())
		assert.Len(t, commitsInfo, len(testCase.ExpectedIDs))
		for _, commitInfo := range commitsInfo {
			entry := commitInfo.Entry
			commit := commitInfo.Commit
			expectedID, ok := testCase.ExpectedIDs[entry.Name()]
			if !assert.True(t, ok) {
				continue
			}
			assert.Equal(t, expectedID, commit.ID.String())
		}
	}
}

func TestEntries_GetCommitsInfo(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := openRepositoryWithDefaultContext(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	testGetCommitsInfo(t, bareRepo1)

	clonedPath, err := cloneRepo(t, bareRepo1Path)
	if err != nil {
		assert.NoError(t, err)
	}
	clonedRepo1, err := openRepositoryWithDefaultContext(clonedPath)
	if err != nil {
		assert.NoError(t, err)
	}
	defer clonedRepo1.Close()

	testGetCommitsInfo(t, clonedRepo1)
}

func BenchmarkEntries_GetCommitsInfo(b *testing.B) {
	type benchmarkType struct {
		url  string
		name string
	}

	benchmarks := []benchmarkType{
		{url: "https://github.com/go-gitea/gitea.git", name: "gitea"},
		{url: "https://github.com/ethantkoenig/manyfiles.git", name: "manyfiles"},
		{url: "https://github.com/moby/moby.git", name: "moby"},
		{url: "https://github.com/golang/go.git", name: "go"},
		{url: "https://github.com/torvalds/linux.git", name: "linux"},
	}

	doBenchmark := func(benchmark benchmarkType) {
		var commit *Commit
		var entries Entries
		var repo *Repository
		repoPath, err := cloneRepo(b, benchmark.url)
		if err != nil {
			b.Fatal(err)
		}

		if repo, err = openRepositoryWithDefaultContext(repoPath); err != nil {
			b.Fatal(err)
		}
		defer repo.Close()

		if commit, err = repo.GetBranchCommit("master"); err != nil {
			b.Fatal(err)
		} else if entries, err = commit.Tree.ListEntries(); err != nil {
			b.Fatal(err)
		}
		entries.Sort()
		b.ResetTimer()
		b.Run(benchmark.name, func(b *testing.B) {
			for b.Loop() {
				_, _, err := entries.GetCommitsInfo(b.Context(), commit, "")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	for _, benchmark := range benchmarks {
		doBenchmark(benchmark)
	}
}

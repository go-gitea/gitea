// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"gitea.dev/modules/cache"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testWriteCountingCache struct {
	cache.StringCache
	writes int
}

func (c *testWriteCountingCache) Put(string, string, int64) error {
	c.writes++
	return nil
}

type testLogCommit struct {
	parents []int
	time    int
	files   map[string]string
}

func testRange(from, to int) []int {
	values := make([]int, 0, to-from+1)
	for value := from; value <= to; value++ {
		values = append(values, value)
	}
	return values
}

func appendTestLogChain(commits []testLogCommit, times ...int) []testLogCommit {
	for _, commitTime := range times {
		commit := testLogCommit{time: commitTime}
		if len(commits) > 0 {
			commit.parents = []int{len(commits) - 1}
		}
		commits = append(commits, commit)
	}
	return commits
}

func newTestLogRepo(t *testing.T, histories ...[]testLogCommit) (*Repository, [][]string) {
	repoDir := t.TempDir()
	require.NoError(t, gitcmd.NewCommand("init", "--bare").AddDynamicArguments(repoDir).Run(t.Context()))
	var stdin strings.Builder
	marks := 0
	for _, commits := range histories {
		stdin.WriteString("reset refs/heads/main\n")
		for i, commit := range commits {
			fmt.Fprintf(&stdin, "commit refs/heads/main\nmark :%d\ncommitter User <user@example.com> %d +0000\ndata 0\n", marks+i+1, 1714310400+commit.time)
			for j, parent := range commit.parents {
				fmt.Fprintf(&stdin, "%s :%d\n", util.Iif(j == 0, "from", "merge"), marks+parent+1)
			}
			for name, content := range commit.files {
				if content == "" {
					fmt.Fprintf(&stdin, "D %s\n", name)
				} else {
					fmt.Fprintf(&stdin, "M 100644 inline %s\ndata %d\n%s\n", name, len(content), content)
				}
			}
		}
		marks += len(commits)
	}
	for i := range marks {
		fmt.Fprintf(&stdin, "get-mark :%d\n", i+1)
	}
	stdout, _, runErr := gitcmd.NewCommand("fast-import", "--quiet").WithDir(repoDir).WithStdinBytes([]byte(stdin.String())).RunStdString(t.Context())
	require.NoError(t, runErr)
	repo, err := OpenRepository(t.Context(), gitrepo.RepositoryManaged("repo", repoDir))
	require.NoError(t, err)
	t.Cleanup(func() { repo.Close() })
	commitIDs := strings.Fields(stdout)
	historyIDs := make([][]string, 0, len(histories))
	for _, commits := range histories {
		historyIDs, commitIDs = append(historyIDs, commitIDs[:len(commits)]), commitIDs[len(commits):]
	}
	return repo, historyIDs
}

func TestWalkGitLogLinearHistory(t *testing.T) {
	merged := append(appendTestLogChain(nil, 1, 2), testLogCommit{parents: []int{0}, time: 3}, testLogCommit{parents: []int{1, 2}, time: 4})
	certificateCases := []struct {
		name    string
		commits []testLogCommit
		want    []int
	}{
		{"includes root", appendTestLogChain(nil, testRange(1, 40)...), testRange(0, 39)},
		{"stops at equal time", appendTestLogChain(nil, append([]int{1, 2, 3, 3}, testRange(5, 40)...)...), testRange(4, 39)},
		{"stops at older time", appendTestLogChain(nil, append([]int{1, 2, 4, 3}, testRange(5, 40)...)...), testRange(4, 39)},
		{"stops at commit-graph time limit", appendTestLogChain(nil, append(testRange(1, 39), commitGraphTimeLimit-1714310400)...), nil},
		{"stops at merge", appendTestLogChain(slices.Clone(merged), testRange(5, 40)...), testRange(4, 39)},
		{"skips near merge", appendTestLogChain(slices.Clone(merged), testRange(5, 35)...), nil},
		{"stops at count limit", appendTestLogChain(nil, testRange(1, 1001)...), testRange(2, 1000)},
	}
	files := map[string]string{}
	for name := range strings.FieldsSeq("w x y z f0 f1 f2 f3 f4 f5 f6 f7") {
		files[name] = "0"
	}
	walkCases := []struct {
		name    string
		commits []testLogCommit
		head    map[string]string
		want    map[string]int
	}{
		{
			name: "hidden merge",
			commits: []testLogCommit{
				{files: files},
				{parents: []int{0}, time: 1, files: map[string]string{"z": "1"}},
				{parents: []int{1}, time: 2, files: map[string]string{"z": "0", "x": "1"}},
				{parents: []int{0}, time: 3, files: map[string]string{"x": "2"}},
				{parents: []int{2, 3}, time: 4, files: map[string]string{"x": "2"}},
				{parents: []int{4}, time: 5, files: map[string]string{"w": "1"}},
			},
			head: map[string]string{"x": "3", "f0": "1", "f1": "1"},
			want: map[string]int{"w": 5, "z": 2},
		},
		{
			name:    "clock skew",
			commits: []testLogCommit{{time: 2, files: files}, {parents: []int{0}, time: 1, files: map[string]string{"x": "1"}}},
			head:    map[string]string{"y": "1", "f0": "1", "f1": "1"},
			want:    map[string]int{"x": 1},
		},
		{
			name:    "log.follow",
			commits: []testLogCommit{{files: map[string]string{"old": "same", "other": "0"}}, {parents: []int{0}, time: 1, files: map[string]string{"old": "", "target": "same"}}},
			head:    map[string]string{"other": "1"},
			want:    map[string]int{"target": 1},
		},
	}

	histories := make([][]testLogCommit, 0, len(certificateCases)+len(walkCases))
	for _, testCase := range certificateCases {
		histories = append(histories, testCase.commits)
	}
	for _, testCase := range walkCases {
		commits := appendTestLogChain(slices.Clone(testCase.commits), testRange(6, 38)...)
		commits[len(commits)-1].files = testCase.head
		histories = append(histories, commits)
	}
	repo, historyIDs := newTestLogRepo(t, histories...)
	require.NoError(t, gitcmd.NewCommand("config", "log.follow", "true").WithRepo(repo).Run(t.Context()))

	for i, testCase := range certificateCases {
		t.Run(testCase.name, func(t *testing.T) {
			commitIDs := historyIDs[i]
			var want []string
			for _, index := range testCase.want {
				want = append(want, commitIDs[index])
			}
			history := linearLogHistory(t.Context(), repo, commitIDs[len(commitIDs)-1])
			assert.ElementsMatch(t, want, slices.Collect(maps.Keys(history)))
		})
	}
	for i, testCase := range walkCases {
		t.Run(testCase.name, func(t *testing.T) {
			commitIDs := historyIDs[len(certificateCases)+i]
			head, err := repo.GetCommit(t.Context(), commitIDs[len(commitIDs)-1])
			require.NoError(t, err)
			entries, err := head.Tree().ListEntries(t.Context(), repo)
			require.NoError(t, err)
			want := map[string]string{"": head.ID.String()}
			for _, entry := range entries {
				want[entry.Name()] = util.Iif(testCase.head[entry.Name()] != "", head.ID.String(), commitIDs[testCase.want[entry.Name()]])
			}
			writeCounter := &testWriteCountingCache{}
			repo.LastCommitCache.cache = writeCounter
			got, err := walkGitLog(t.Context(), repo, head, "")
			require.NoError(t, err)
			assert.Equal(t, want, got)
			assert.Equal(t, len(want), writeCounter.writes)
		})
	}
}

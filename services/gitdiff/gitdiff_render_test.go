// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"path/filepath"
	"strings"
	"testing"

	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDiffForRender(t *testing.T) {
	repoDir := filepath.Join(t.TempDir(), "temp-repo")
	require.NoError(t, git.InitRepositoryLocal(t.Context(), repoDir, false, git.Sha1ObjectFormat.Name()))

	contentLeft := strings.Repeat("a\n", 20) +
		"mark1\n" +
		strings.Repeat("b\n", 10) +
		"mark2\n" +
		strings.Repeat("c\n", 40) +
		"mark3\n" +
		strings.Repeat("a\n", 30)
	contentRight := strings.Repeat("a\n", 20-1) +
		"mark1-x\n" +
		strings.Repeat("b\n", 10+3) +
		"mark2-x\n" +
		strings.Repeat("c\n", 40-5) +
		"mark3-x\n" +
		strings.Repeat("a\n", 30)

	gitRepo, err := git.OpenRepositoryLocal(t.Context(), repoDir)
	require.NoError(t, err)
	defer gitRepo.Close()

	require.NoError(t, git.ForceFastImport(t.Context(), gitRepo, []git.FastImportCommit{
		{Ref: "refs/heads/b1", Files: []git.FastImportFile{{Path: "foo.txt", Content: contentLeft}}},
		{Ref: "refs/heads/b2", Files: []git.FastImportFile{{Path: "foo.txt", Content: contentRight}}},
	}))

	beforeCommit, err := gitRepo.GetBranchCommit(t.Context(), "b1")
	require.NoError(t, err)
	afterCommit, err := gitRepo.GetBranchCommit(t.Context(), "b2")
	require.NoError(t, err)

	diff, err := GetDiffForRender(t.Context(), "/any/repo-link", gitRepo, &DiffOptions{
		BeforeCommitID:    beforeCommit.ID.String(),
		AfterCommitID:     afterCommit.ID.String(),
		MaxLines:          setting.Git.MaxGitDiffLines,
		MaxLineCharacters: setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:          setting.Git.MaxGitDiffFiles,
	})
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	type section struct {
		ExpandDirection string

		LastLeftIdx   int
		LastRightIdx  int
		LeftIdx       int
		RightIdx      int
		LeftHunkSize  int
		RightHunkSize int
	}
	expectedSections := []section{
		{
			ExpandDirection: "up",
			LeftIdx:         17,
			RightIdx:        17,
			LeftHunkSize:    8,
			RightHunkSize:   7,
		},
		{
			ExpandDirection: "single",
			LastLeftIdx:     24,
			LastRightIdx:    23,
			LeftIdx:         29,
			RightIdx:        28,
			LeftHunkSize:    7,
			RightHunkSize:   10,
		},
		{
			ExpandDirection: "updown",
			LastLeftIdx:     35,
			LastRightIdx:    37,
			LeftIdx:         65,
			RightIdx:        67,
			LeftHunkSize:    12,
			RightHunkSize:   7,
		},
		{
			ExpandDirection: "down",
			LastLeftIdx:     76,
			LastRightIdx:    73,
			LeftIdx:         103, // left has 103 lines
			RightIdx:        100, // right has 100 lines
		},
	}
	for idx, exp := range expectedSections {
		secLine := diff.Files[0].Sections[idx].Lines[0] // line 0 should be the "section info"
		actual := section{
			ExpandDirection: secLine.GetExpandDirection(),
			LastLeftIdx:     secLine.SectionInfo.LastLeftIdx,
			LastRightIdx:    secLine.SectionInfo.LastRightIdx,
			LeftIdx:         secLine.SectionInfo.LeftIdx,
			RightIdx:        secLine.SectionInfo.RightIdx,
			LeftHunkSize:    secLine.SectionInfo.LeftHunkSize,
			RightHunkSize:   secLine.SectionInfo.RightHunkSize,
		}
		assert.Equal(t, exp, actual, "idx=%d", idx)
	}
}

// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildCherryPickTestRepo creates a temporary git repository with the following topology:
//
//	main:    A
//	feature: A → B          (B = feature file commit)
//	staging: A → M1         (M1 = merge commit "feature into staging")
//	develop: A → M2         (M2 = merge commit "feature into develop")
//
// It returns the path to the repository, the SHA of M1 (head of staging) and
// the SHA of M2 (head of develop).
func buildCherryPickTestRepo(t *testing.T) (repoPath, m1SHA, m2SHA string) {
	t.Helper()

	repoPath = t.TempDir()

	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = repoPath
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Gitea Test",
			"GIT_AUTHOR_EMAIL=test@gitea.com",
			"GIT_COMMITTER_NAME=Gitea Test",
			"GIT_COMMITTER_EMAIL=test@gitea.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed:\n%s", args, out)
		return strings.TrimSpace(string(out))
	}

	// Initialise repository
	git("init", "-b", "main")
	git("config", "user.email", "test@gitea.com")
	git("config", "user.name", "Gitea Test")

	// Commit A on main
	err := os.WriteFile(filepath.Join(repoPath, "base.txt"), []byte("base content\n"), 0o644)
	require.NoError(t, err)
	git("add", "base.txt")
	git("commit", "-m", "Commit A")

	// Create staging and develop branches pointing at Commit A
	git("checkout", "-b", "staging")
	git("checkout", "main")
	git("checkout", "-b", "develop")
	git("checkout", "main")

	// Create feature branch and add Commit B
	git("checkout", "-b", "feature")
	err = os.WriteFile(filepath.Join(repoPath, "feature.txt"), []byte("feature content\n"), 0o644)
	require.NoError(t, err)
	git("add", "feature.txt")
	git("commit", "-m", "Commit B")

	// Merge feature into staging → M1
	git("checkout", "staging")
	git("merge", "--no-ff", "feature", "-m", "Merge feature into staging")
	m1SHA = git("rev-parse", "HEAD")

	// Merge feature into develop → M2
	git("checkout", "develop")
	git("merge", "--no-ff", "feature", "-m", "Merge feature into develop")
	m2SHA = git("rev-parse", "HEAD")

	return repoPath, m1SHA, m2SHA
}

// TestShowPrettyFormatLogToListSkipEquivalent tests the patch-equivalence aware log against
// the scenario described in issue #37383:
//
//   - Commit B's patch lands in staging via M1 (a merge commit).
//   - M2 merges the same feature branch into develop.
//   - ShowPrettyFormatLogToListSkipEquivalent("staging", M2) must return only M2:
//     B is already present in staging so it is skipped; M2 is a merge commit and is kept.
func TestShowPrettyFormatLogToListSkipEquivalent(t *testing.T) {
	repoPath, _, m2SHA := buildCherryPickTestRepo(t)

	ctx := t.Context()
	repo, err := OpenRepository(ctx, repoPath)
	require.NoError(t, err)
	defer repo.Close()

	// Primary assertion: B's patch is already present in staging via M1, so it must be
	// skipped. M2 is a merge commit with no equivalent on the base side, so it is kept.
	filteredCommits, err := repo.ShowPrettyFormatLogToListSkipEquivalent(ctx, "staging", m2SHA)
	require.NoError(t, err)
	require.Len(t, filteredCommits, 1, "expected only M2: B is already in staging via M1")
	assert.Equal(t, m2SHA, filteredCommits[0].ID.String(), "the remaining commit must be M2")

	// Contrast assertion: a plain log over the same range returns both M2 and B,
	// confirming that the filter is what removes B — not an empty range.
	mergeBaseCmd := exec.CommandContext(ctx, "git", "merge-base", "staging", m2SHA)
	mergeBaseCmd.Dir = repoPath
	mergeBaseOut, err := mergeBaseCmd.Output()
	require.NoError(t, err)
	mergeBase := strings.TrimSpace(string(mergeBaseOut))

	plainCommits, err := repo.ShowPrettyFormatLogToList(ctx, mergeBase+".."+m2SHA)
	require.NoError(t, err)
	assert.NotEmpty(t, plainCommits, "plain log must be non-empty")
}

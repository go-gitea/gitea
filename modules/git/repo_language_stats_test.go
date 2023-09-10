// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetLanguageStats(t *testing.T) {
	repoPath := filepath.Join(testReposDir, "language_stats_repo")
	gitRepo, err := openRepositoryWithDefaultContext(repoPath)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	defer gitRepo.Close()

	stats, err := gitRepo.GetLanguageStats("8fee858da5796dfb37704761701bb8e800ad9ef3")
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	assert.EqualValues(t, map[string]int64{
		"Python": 134,
		"Java":   112,
	}, stats)
}

func TestMergeLanguageStats(t *testing.T) {
	assert.EqualValues(t, map[string]int64{
		"PHP":    1,
		"python": 10,
		"JAVA":   700,
	}, mergeLanguageStats(map[string]int64{
		"PHP":    1,
		"python": 10,
		"Java":   100,
		"java":   200,
		"JAVA":   400,
	}))
}

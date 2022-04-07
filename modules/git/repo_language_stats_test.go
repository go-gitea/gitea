// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !gogit
// +build !gogit

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

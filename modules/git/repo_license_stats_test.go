// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GetLicenseStats(t *testing.T) {
	// TODO: add the repo
	repoPath := filepath.Join(testReposDir, "license_stats_repo")
	gitRepo, err := openRepositoryWithDefaultContext(repoPath)
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	defer gitRepo.Close()

	// TODO: add the LICENSE file
	stats, err := gitRepo.GetLicenseStats("8fee858da5796dfb37704761701bb8e800ad9ef3")
	if !assert.NoError(t, err) {
		t.Fatal()
	}

	// TODO: fix check
	assert.EqualValues(t, map[string]int64{
		"Python": 134,
		"Java":   112,
	}, stats)
}

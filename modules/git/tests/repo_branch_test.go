// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tests

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git/service"
	"github.com/stretchr/testify/assert"
)

func TestRepository_GetBranches(t *testing.T) {
	RunTestPerProvider(t, func(service service.GitService, t *testing.T) {
		bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
		bareRepo1, err := service.OpenRepository(bareRepo1Path)
		assert.NoError(t, err)
		defer bareRepo1.Close()

		branches, err := bareRepo1.GetBranches()

		assert.NoError(t, err)
		assert.Len(t, branches, 3)
		assert.ElementsMatch(t, []string{"branch1", "branch2", "master"}, branches)
	})
}

func BenchmarkRepository_GetBranches(b *testing.B) {
	RunBenchmarkPerProvider(b, func(service service.GitService, b *testing.B) {
		bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
		bareRepo1, err := service.OpenRepository(bareRepo1Path)
		if err != nil {
			b.Fatal(err)
		}
		defer bareRepo1.Close()

		for i := 0; i < b.N; i++ {
			_, err := bareRepo1.GetBranches()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

var testReposDir = "tests/repos/"

func TestVerifyCommits(t *testing.T) {
	unittest.PrepareTestEnv(t)

	gitRepo, err := git.OpenRepository(context.Background(), testReposDir+"repo1_hook_verification")
	assert.NoError(t, err)

	testCases := []struct {
		base, head string
		verified   bool
	}{
		{"7d92f8f0cf7b989acf2c56a9a4387f5b39a92c41", "64e643f86a640d97f95820b43aeb28631042c629", true},
		{git.EmptySHA, "451c1608ab9c5277fdc73ea68aea711c046ac7ac", true}, // New branch with verified commit
		{"e05ec071e40a53cd4785345a99fd12f5dbb6a777", "7d92f8f0cf7b989acf2c56a9a4387f5b39a92c41", false},
		{git.EmptySHA, "1128cd8e82948ea10b4932b2481210610604ce7e", false}, // New branch with unverified commit
	}

	for _, tc := range testCases {
		err = verifyCommits(tc.base, tc.head, gitRepo, nil)
		if tc.verified {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	}
}

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

var testReposDir = "tests/repos/"

func TestVerifyCommits(t *testing.T) {
	unittest.PrepareTestEnv(t)

	gitRepo, err := git.OpenRepository(t.Context(), testReposDir+"repo1_hook_verification")
	defer gitRepo.Close()
	assert.NoError(t, err)

	objectFormat, err := gitRepo.GetObjectFormat()
	assert.NoError(t, err)

	testCases := []struct {
		base, head string
		verified   bool
	}{
		{"72920278f2f999e3005801e5d5b8ab8139d3641c", "d766f2917716d45be24bfa968b8409544941be32", true},
		{objectFormat.EmptyObjectID().String(), "93eac826f6188f34646cea81bf426aa5ba7d3bfe", true}, // New branch with verified commit
		{"9779d17a04f1e2640583d35703c62460b2d86e0a", "72920278f2f999e3005801e5d5b8ab8139d3641c", false},
		{objectFormat.EmptyObjectID().String(), "9ce3f779ae33f31fce17fac3c512047b75d7498b", false}, // New branch with unverified commit
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

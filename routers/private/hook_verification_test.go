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

func TestVerifyCommits(t *testing.T) {
	unittest.PrepareTestEnv(t)

	gitRepo, err := git.OpenRepository(context.Background(), ".")
	assert.NoError(t, err)
	defer gitRepo.Close()

	err = verifyCommits(git.EmptySHA, "9c5c60143975a120bf70ac4aed34bf903ef19db6", gitRepo, nil)
	assert.NoError(t, err)
}

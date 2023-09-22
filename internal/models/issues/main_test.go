// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	issues_model "code.gitea.io/gitea/internal/models/issues"
	"code.gitea.io/gitea/internal/models/unittest"

	_ "code.gitea.io/gitea/internal/models"
	_ "code.gitea.io/gitea/internal/models/actions"
	_ "code.gitea.io/gitea/internal/models/activities"
	_ "code.gitea.io/gitea/internal/models/repo"
	_ "code.gitea.io/gitea/internal/models/user"

	"github.com/stretchr/testify/assert"
)

func TestFixturesAreConsistent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	unittest.CheckConsistencyFor(t,
		&issues_model.Issue{},
		&issues_model.PullRequest{},
		&issues_model.Milestone{},
		&issues_model.Label{},
	)
}

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}

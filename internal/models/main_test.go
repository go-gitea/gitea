// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"

	activities_model "code.gitea.io/gitea/internal/models/activities"
	"code.gitea.io/gitea/internal/models/organization"
	repo_model "code.gitea.io/gitea/internal/models/repo"
	"code.gitea.io/gitea/internal/models/unittest"
	user_model "code.gitea.io/gitea/internal/models/user"

	_ "code.gitea.io/gitea/internal/models/actions"
	_ "code.gitea.io/gitea/internal/models/system"

	"github.com/stretchr/testify/assert"
)

// TestFixturesAreConsistent assert that test fixtures are consistent
func TestFixturesAreConsistent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	unittest.CheckConsistencyFor(t,
		&user_model.User{},
		&repo_model.Repository{},
		&organization.Team{},
		&activities_model.Action{})
}

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{})
}

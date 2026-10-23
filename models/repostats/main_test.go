// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repostats

import (
	"testing"

	activities_model "codenest.dev/models/activities"
	"codenest.dev/models/organization"
	repo_model "codenest.dev/models/repo"
	"codenest.dev/models/unittest"
	user_model "codenest.dev/models/user"

	_ "codenest.dev/models/actions"
	_ "codenest.dev/models/system"

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
	unittest.MainTest(m)
}

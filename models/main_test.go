// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"testing"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models/system"

	"github.com/stretchr/testify/assert"
)

func init() {
	setting.SetCustomPathAndConf("", "", "")
	setting.LoadForTest()
}

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
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: "..",
	})
}

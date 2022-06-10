// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues_test

import (
	"path/filepath"
	"testing"

	_ "code.gitea.io/gitea/models"
	issues_model "code.gitea.io/gitea/models/issues"
	_ "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	_ "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func init() {
	setting.SetCustomPathAndConf("", "", "")
	setting.LoadForTest()
}

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
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
	})
}

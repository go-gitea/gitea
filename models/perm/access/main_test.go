// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package access

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models/repo"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", "..", ".."),
		FixtureFiles: []string{
			"access.yml",
			"user.yml",
			"repository.yml",
			"collaboration.yml",
			"org_user.yml",
			"repo_unit.yml",
			"team_user.yml",
			"team_repo.yml",
			"team.yml",
			"team_unit.yml",
		},
	})
}

// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package organization

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", ".."),
		"user.yml",
		"org_user.yml",
		"team.yml",
		"team_repo.yml",
		"team_unit.yml",
		"team_user.yml",
		"repository.yml",
	)
}

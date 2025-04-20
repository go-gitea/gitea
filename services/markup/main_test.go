// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		FixtureFiles: []string{"user.yml", "repository.yml", "access.yml", "repo_unit.yml", "issue.yml"},
	})
}

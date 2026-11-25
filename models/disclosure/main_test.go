// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package disclosure

import (
	"testing"

	"gitea.dev/models/unittest"

	_ "gitea.dev/models/repo"
	_ "gitea.dev/models/user"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		FixtureFiles: []string{
			"repository.yml",
			"user.yml",
		},
		SkipRepoFixtures: true,
	})
}

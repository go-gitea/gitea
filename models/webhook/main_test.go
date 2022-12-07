// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
		FixtureFiles: []string{
			"webhook.yml",
			"hook_task.yml",
		},
	})
}

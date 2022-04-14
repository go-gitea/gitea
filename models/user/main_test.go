// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
		FixtureFiles: []string{
			"email_address.yml",
			"user_redirect.yml",
			"follow.yml",
			"user_open_id.yml",
			"two_factor.yml",
			"oauth2_application.yml",
			"user.yml",
		},
	})
}

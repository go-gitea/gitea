// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package login

import (
	"code.gitea.io/gitea/models/unittest"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", ".."),
		"login_source.yml",
		"oauth2_application.yml",
		"oauth2_authorization_code.yml",
		"oauth2_grant.yml",
		"u2f_registration.yml",
	)
}

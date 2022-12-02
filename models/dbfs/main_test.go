// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dbfs

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models"
	_ "code.gitea.io/gitea/models/repo"
	_ "code.gitea.io/gitea/models/user"
)

func init() {
	setting.SetCustomPathAndConf("", "", "")
	setting.LoadForTest()
}

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
	})
}

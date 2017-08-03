// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	_ "github.com/mattn/go-sqlite3" // for the test engine
)

func TestMain(m *testing.M) {
	if err := models.CreateTestEngine("../../models/fixtures/"); err != nil {
		fmt.Printf("Error creating test engine: %v\n", err)
		os.Exit(1)
	}

	setting.AppURL = "https://try.gitea.io/"
	setting.RunUser = "runuser"
	setting.SSH.Port = 3000
	setting.SSH.Domain = "try.gitea.io"
	setting.RepoRootPath = filepath.Join(os.TempDir(), "repos")
	setting.AppDataPath = filepath.Join(os.TempDir(), "appdata")

	os.Exit(m.Run())
}

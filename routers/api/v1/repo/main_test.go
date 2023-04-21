// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	webhook_service "code.gitea.io/gitea/services/webhook"
)

func TestMain(m *testing.M) {
	setting.InitProviderAndLoadCommonSettingsForTest()
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", "..", "..", ".."),
		SetUp:         webhook_service.Init,
	})
}

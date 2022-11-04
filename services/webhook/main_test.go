// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/modules/setting"

	_ "code.gitea.io/gitea/models"
)

func TestMain(m *testing.M) {
	setting.LoadForTest()
	setting.NewQueueService()

	// for tests, allow only loopback IPs
	setting.Webhook.AllowedHostList = hostmatcher.MatchBuiltinLoopback
	unittest.MainTest(m, &unittest.TestOptions{
		GiteaRootPath: filepath.Join("..", ".."),
		SetUp:         Init,
	})
}

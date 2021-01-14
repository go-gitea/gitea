// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
)

func TestRepoWatch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// Test round-trip auto-watch
		setting.Service.AutoWatchOnChanges = true
		session := loginUser(t, "user2")
		models.AssertNotExistsBean(t, &models.Watch{UserID: 2, RepoID: 3})
		testEditFile(t, session, "user3", "repo3", "master", "README.md", "Hello, World (Edited for watch)\n")
		models.AssertExistsAndLoadBean(t, &models.Watch{UserID: 2, RepoID: 3, Mode: models.RepoWatchModeAuto})
	})
}

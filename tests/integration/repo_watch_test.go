// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/setting/config"
	"github.com/stretchr/testify/assert"
)

func TestRepoWatch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		// Test round-trip auto-watch

		assert.NoError(t, system.SetSettings(t.Context(), map[string]string{
			setting.Config().Service.AutoWatchOnChanges.DynKey(): "true",
		}))
		config.GetDynGetter().InvalidateCache()

		session := loginUser(t, "user2")
		unittest.AssertNotExistsBean(t, &repo_model.Watch{UserID: 2, RepoID: 3})
		testEditFile(t, session, "org3", "repo3", "master", "README.md", "Hello, World (Edited for watch)\n")
		unittest.AssertExistsAndLoadBean(t, &repo_model.Watch{UserID: 2, RepoID: 3, Mode: repo_model.WatchModeAuto})
	})
}

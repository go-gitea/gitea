// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/migrations"

	"github.com/stretchr/testify/assert"
)

func TestDumpRestore(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		AllowLocalNetworks := setting.Migrations.AllowLocalNetworks
		setting.Migrations.AllowLocalNetworks = true
		AppVer := setting.AppVer
		// Gitea SDK (go-sdk) need to parse the AppVer from server response, so we must set it to a valid version string.
		setting.AppVer = "1.16.0"
		defer func() {
			setting.Migrations.AllowLocalNetworks = AllowLocalNetworks
			setting.AppVer = AppVer
		}()

		assert.NoError(t, migrations.Init())

		reponame := "repo1"

		basePath, err := os.MkdirTemp("", reponame)
		assert.NoError(t, err)
		defer util.RemoveAll(basePath)

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame}).(*repo_model.Repository)
		repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)
		session := loginUser(t, repoOwner.Name)
		token := getTokenForLoggedInUser(t, session)

		//
		// Phase 1: dump repo1 from the Gitea instance to the filesystem
		//

		ctx := context.Background()
		var opts = migrations.MigrateOptions{
			GitServiceType: structs.GiteaService,
			Issues:         true,
			Comments:       true,
			AuthToken:      token,
			CloneAddr:      repo.CloneLink().HTTPS,
			RepoName:       reponame,
		}
		err = migrations.DumpRepository(ctx, basePath, repoOwner.Name, opts)
		assert.NoError(t, err)

		//
		// Verify desired side effects of the dump
		//
		d := filepath.Join(basePath, repo.OwnerName, repo.Name)
		for _, f := range []string{"repo.yml", "topic.yml", "issue.yml"} {
			assert.FileExists(t, filepath.Join(d, f))
		}

		//
		// Phase 2: restore from the filesystem to the Gitea instance in restoredrepo
		//

		newreponame := "restoredrepo"
		err = migrations.RestoreRepository(ctx, d, repo.OwnerName, newreponame, []string{"issues", "comments"})
		assert.NoError(t, err)

		newrepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: newreponame}).(*repo_model.Repository)

		//
		// Phase 3: dump restoredrepo from the Gitea instance to the filesystem
		//
		opts.RepoName = newreponame
		opts.CloneAddr = newrepo.CloneLink().HTTPS
		err = migrations.DumpRepository(ctx, basePath, repoOwner.Name, opts)
		assert.NoError(t, err)

		//
		// Verify the dump of restoredrepo is the same as the dump of repo1
		//
		newd := filepath.Join(basePath, newrepo.OwnerName, newrepo.Name)
		beforeBytes, err := os.ReadFile(filepath.Join(d, "repo.yml"))
		assert.NoError(t, err)
		before := strings.ReplaceAll(string(beforeBytes), reponame, newreponame)
		after, err := os.ReadFile(filepath.Join(newd, "repo.yml"))
		assert.NoError(t, err)
		assert.EqualValues(t, before, string(after))
	})
}

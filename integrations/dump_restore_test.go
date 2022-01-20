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
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/migrations"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
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
			Labels:         true,
			Milestones:     true,
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
		for _, f := range []string{"repo.yml", "topic.yml", "label.yml", "milestone.yml", "issue.yml"} {
			assert.FileExists(t, filepath.Join(d, f))
		}

		//
		// Phase 2: restore from the filesystem to the Gitea instance in restoredrepo
		//

		newreponame := "restoredrepo"
		err = migrations.RestoreRepository(ctx, d, repo.OwnerName, newreponame, []string{"labels", "milestones", "issues", "comments"})
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
		for _, filename := range []string{"repo.yml", "label.yml", "milestone.yml"} {
			beforeBytes, err := os.ReadFile(filepath.Join(d, filename))
			assert.NoError(t, err)
			before := strings.ReplaceAll(string(beforeBytes), reponame, newreponame)
			after, err := os.ReadFile(filepath.Join(newd, filename))
			assert.NoError(t, err)
			assert.EqualValues(t, before, string(after))
		}

		beforeBytes, err := os.ReadFile(filepath.Join(d, "issue.yml"))
		assert.NoError(t, err)
		var before = make([]*base.Issue, 0, 10)
		assert.NoError(t, yaml.Unmarshal(beforeBytes, &before))
		afterBytes, err := os.ReadFile(filepath.Join(newd, "issue.yml"))
		assert.NoError(t, err)
		var after = make([]*base.Issue, 0, 10)
		assert.NoError(t, yaml.Unmarshal(afterBytes, &after))

		assert.EqualValues(t, len(before), len(after))
		if len(before) == len(after) {
			for i := 0; i < len(before); i++ {
				assert.EqualValues(t, before[i].Number, after[i].Number)
				assert.EqualValues(t, before[i].Title, after[i].Title)
				assert.EqualValues(t, before[i].Content, after[i].Content)
				assert.EqualValues(t, before[i].Ref, after[i].Ref)
				assert.EqualValues(t, before[i].Milestone, after[i].Milestone)
				assert.EqualValues(t, before[i].State, after[i].State)
				assert.EqualValues(t, before[i].IsLocked, after[i].IsLocked)
				assert.EqualValues(t, before[i].Created, after[i].Created)
				assert.EqualValues(t, before[i].Updated, after[i].Updated)
				assert.EqualValues(t, before[i].Labels, after[i].Labels)
			}
		}
	})
}

// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"context"
	"net/url"
	"testing"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/f3/util"
	"lab.forgefriends.org/friendlyforgeformat/gof3"
	f3_forges "lab.forgefriends.org/friendlyforgeformat/gof3/forges"
	f3_f3 "lab.forgefriends.org/friendlyforgeformat/gof3/forges/f3"
	f3_gitea "lab.forgefriends.org/friendlyforgeformat/gof3/forges/gitea"
	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	f3_util "lab.forgefriends.org/friendlyforgeformat/gof3/util"

	"github.com/stretchr/testify/assert"
)

func TestF3(t *testing.T) {
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

		//
		// Step 1: create a fixture
		//
		fixtureNewF3Forge := func(t *testing.T, user *format.User) *f3_forges.ForgeRoot {
			root := f3_forges.NewForgeRoot(&f3_f3.Options{
				Options: gof3.Options{
					Configuration: gof3.Configuration{
						Directory: t.TempDir(),
					},
					Features: gof3.AllFeatures,
				},
				Remap: true,
			})
			root.SetContext(context.Background())
			return root
		}
		fixture := f3_forges.NewFixture(t, f3_forges.FixtureForgeFactory{Fun: fixtureNewF3Forge, RootRequired: false})
		fixture.NewUser()
		fixture.NewMilestone()
		fixture.NewLabel()
		fixture.NewIssue()
		fixture.NewTopic()
		fixture.NewRepository()
		fixture.NewPullRequest()
		fixture.NewRelease()
		fixture.NewAsset()
		fixture.NewIssueComment()
		fixture.NewPullRequestComment()
		fixture.NewReview()
		fixture.NewIssueReaction()
		fixture.NewCommentReaction()

		//
		// Step 2: mirror the fixture into Gitea
		//
		doer, err := user_model.GetAdminUser()
		assert.NoError(t, err)

		giteaLocal := util.GiteaForgeRoot(context.Background(), gof3.AllFeatures, doer)
		giteaLocal.Forge.Mirror(fixture.Forge)

		//
		// Step 3: mirror Gitea into F3
		//
		adminUsername := "user1"
		giteaAPI := f3_forges.NewForgeRootFromDriver(&f3_gitea.Gitea{}, &f3_gitea.Options{
			Options: gof3.Options{
				Configuration: gof3.Configuration{
					URL:       setting.AppURL,
					Directory: t.TempDir(),
				},
				Features: gof3.AllFeatures,
			},
			AuthToken: getUserToken(t, adminUsername),
		})
		giteaAPI.SetContext(context.Background())

		f3 := f3_forges.FixtureNewF3Forge(t, nil)
		apiForge := giteaAPI.Forge
		apiUser := apiForge.Users.GetFromFormat(&format.User{UserName: fixture.UserFormat.UserName})
		apiProject := apiUser.Projects.GetFromFormat(&format.Project{Name: fixture.ProjectFormat.Name})
		f3.Forge.Mirror(apiForge, apiUser, apiProject)

		//
		// Step 4: verify the fixture and F3 are equivalent
		//
		files := f3_util.Command(context.Background(), "find", f3.GetDirectory())
		assert.Contains(t, files, "/repository/git/hooks")
		assert.Contains(t, files, "/label/")
		assert.Contains(t, files, "/issue/")
		assert.Contains(t, files, "/milestone/")
		assert.Contains(t, files, "/topic/")
		assert.Contains(t, files, "/pull_request/")
		assert.Contains(t, files, "/release/")
		assert.Contains(t, files, "/asset/")
		assert.Contains(t, files, "/comment/")
		assert.Contains(t, files, "/review/")
		assert.Contains(t, files, "/issue_reaction/")
		assert.Contains(t, files, "/comment_reaction/")
		//		f3_util.Command(context.Background(), "cp", "-a", f3.GetDirectory(), "abc")
	})
}

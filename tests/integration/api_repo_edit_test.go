// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// getRepoEditOptionFromRepo gets the options for an existing repo exactly as is
func getRepoEditOptionFromRepo(repo *repo_model.Repository) *api.EditRepoOption {
	name := repo.Name
	description := repo.Description
	website := repo.Website
	private := repo.IsPrivate
	hasIssues := false
	var internalTracker *api.InternalTracker
	var externalTracker *api.ExternalTracker
	if unit, err := repo.GetUnit(db.DefaultContext, unit_model.TypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(db.DefaultContext, unit_model.TypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:           config.ExternalTrackerURL,
			ExternalTrackerFormat:        config.ExternalTrackerFormat,
			ExternalTrackerStyle:         config.ExternalTrackerStyle,
			ExternalTrackerRegexpPattern: config.ExternalTrackerRegexpPattern,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.GetUnit(db.DefaultContext, unit_model.TypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(db.DefaultContext, unit_model.TypeExternalWiki); err == nil {
		hasWiki = true
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: unit.ExternalWikiConfig().ExternalWikiURL,
		}
	}
	defaultBranch := repo.DefaultBranch
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	allowFastForwardOnly := false
	if unit, err := repo.GetUnit(db.DefaultContext, unit_model.TypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
		allowFastForwardOnly = config.AllowFastForwardOnly
	}
	archived := repo.IsArchived
	hasProjects := false
	var projectsMode *string
	if unit, err := repo.GetUnit(db.DefaultContext, unit_model.TypeProjects); err == nil && unit != nil {
		hasProjects = true
		pm := string(unit.ProjectsConfig().ProjectsMode)
		projectsMode = &pm
	}
	hasCode := repo.UnitEnabled(db.DefaultContext, unit_model.TypeCode)
	hasPackages := repo.UnitEnabled(db.DefaultContext, unit_model.TypePackages)
	hasReleases := repo.UnitEnabled(db.DefaultContext, unit_model.TypeReleases)
	hasActions := false
	if unit, err := repo.GetUnit(db.DefaultContext, unit_model.TypeActions); err == nil && unit != nil {
		hasActions = true
		// TODO: expose action config of repo to api
		// actionsConfig = &api.RepoActionsConfig{
		// 	DisabledWorkflows: unit.ActionsConfig().DisabledWorkflows,
		// }
	}
	return &api.EditRepoOption{
		Name:                      &name,
		Description:               &description,
		Website:                   &website,
		Private:                   &private,
		HasIssues:                 &hasIssues,
		HasProjects:               &hasProjects,
		ProjectsMode:              projectsMode,
		HasCode:                   &hasCode,
		HasPackages:               &hasPackages,
		HasReleases:               &hasReleases,
		HasActions:                &hasActions,
		ExternalTracker:           externalTracker,
		InternalTracker:           internalTracker,
		HasWiki:                   &hasWiki,
		ExternalWiki:              externalWiki,
		DefaultBranch:             &defaultBranch,
		HasPullRequests:           &hasPullRequests,
		IgnoreWhitespaceConflicts: &ignoreWhitespaceConflicts,
		AllowMerge:                &allowMerge,
		AllowRebase:               &allowRebase,
		AllowRebaseMerge:          &allowRebaseMerge,
		AllowSquash:               &allowSquash,
		AllowFastForwardOnly:      &allowFastForwardOnly,
		Archived:                  &archived,
	}
}

// getNewRepoEditOption Gets the options to change everything about an existing repo by adding to strings or changing
// the boolean
func getNewRepoEditOption(opts *api.EditRepoOption) *api.EditRepoOption {
	// Gives a new property to everything
	name := *opts.Name + "renamed"
	description := "new description"
	website := "http://wwww.newwebsite.com"
	private := !*opts.Private
	hasIssues := !*opts.HasIssues
	hasWiki := !*opts.HasWiki
	hasProjects := !*opts.HasProjects
	hasCode := !*opts.HasCode
	hasPackages := !*opts.HasPackages
	hasReleases := !*opts.HasReleases
	hasActions := !*opts.HasActions
	defaultBranch := "master"
	hasPullRequests := !*opts.HasPullRequests
	ignoreWhitespaceConflicts := !*opts.IgnoreWhitespaceConflicts
	allowMerge := !*opts.AllowMerge
	allowRebase := !*opts.AllowRebase
	allowRebaseMerge := !*opts.AllowRebaseMerge
	allowSquash := !*opts.AllowSquash
	archived := !*opts.Archived

	return &api.EditRepoOption{
		Name:                      &name,
		Description:               &description,
		Website:                   &website,
		Private:                   &private,
		DefaultBranch:             &defaultBranch,
		HasIssues:                 &hasIssues,
		HasWiki:                   &hasWiki,
		HasProjects:               &hasProjects,
		HasCode:                   &hasCode,
		HasPackages:               &hasPackages,
		HasReleases:               &hasReleases,
		HasActions:                &hasActions,
		HasPullRequests:           &hasPullRequests,
		IgnoreWhitespaceConflicts: &ignoreWhitespaceConflicts,
		AllowMerge:                &allowMerge,
		AllowRebase:               &allowRebase,
		AllowRebaseMerge:          &allowRebaseMerge,
		AllowSquash:               &allowSquash,
		Archived:                  &archived,
	}
}

func TestAPIRepoEdit(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		bFalse, bTrue := false, true

		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})         // owner of the repo1 & repo16
		org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})          // owner of the repo3, is an org
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})         // owner of neither repos
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})   // public repo
		repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})   // public repo
		repo15 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 15}) // empty repo
		repo16 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16}) // private repo

		// Get user2's token
		session := loginUser(t, user2.Name)
		token2 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		// Get user4's token
		session = loginUser(t, user4.Name)
		token4 := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// Test editing a repo1 which user2 owns, changing name and many properties
		origRepoEditOption := getRepoEditOptionFromRepo(repo1)
		assert.True(t, *origRepoEditOption.HasCode)
		assert.True(t, *origRepoEditOption.HasPackages)
		assert.True(t, *origRepoEditOption.HasProjects)
		assert.True(t, *origRepoEditOption.HasReleases)
		assert.True(t, *origRepoEditOption.HasActions)
		repoEditOption := getNewRepoEditOption(origRepoEditOption)
		req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo1.Name), &repoEditOption).
			AddTokenAuth(token2)
		resp := MakeRequest(t, req, http.StatusOK)
		var repo api.Repository
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		// check response
		assert.Equal(t, *repoEditOption.Name, repo.Name)
		assert.Equal(t, *repoEditOption.Description, repo.Description)
		assert.Equal(t, *repoEditOption.Website, repo.Website)
		assert.Equal(t, *repoEditOption.Archived, repo.Archived)
		// check repo1 from database
		repo1edited := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		repo1editedOption := getRepoEditOptionFromRepo(repo1edited)
		assert.Equal(t, *repoEditOption.Name, *repo1editedOption.Name)
		assert.Equal(t, *repoEditOption.Description, *repo1editedOption.Description)
		assert.Equal(t, *repoEditOption.Website, *repo1editedOption.Website)
		assert.Equal(t, *repoEditOption.Archived, *repo1editedOption.Archived)
		assert.Equal(t, *repoEditOption.Private, *repo1editedOption.Private)
		assert.Equal(t, *repoEditOption.HasWiki, *repo1editedOption.HasWiki)
		assert.Equal(t, *repoEditOption.HasCode, *repo1editedOption.HasCode)
		assert.Equal(t, *repoEditOption.HasPackages, *repo1editedOption.HasPackages)
		assert.Equal(t, *repoEditOption.HasProjects, *repo1editedOption.HasProjects)
		assert.Equal(t, *repoEditOption.HasReleases, *repo1editedOption.HasReleases)
		assert.Equal(t, *repoEditOption.HasActions, *repo1editedOption.HasActions)

		// Test editing repo1 to use internal issue and wiki (default)
		*repoEditOption.HasIssues = true
		repoEditOption.ExternalTracker = nil
		repoEditOption.InternalTracker = &api.InternalTracker{
			EnableTimeTracker:                false,
			AllowOnlyContributorsToTrackTime: false,
			EnableIssueDependencies:          false,
		}
		*repoEditOption.HasWiki = true
		repoEditOption.ExternalWiki = nil
		url := fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, *repoEditOption.Name)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		// check repo1 was written to database
		repo1edited = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		repo1editedOption = getRepoEditOptionFromRepo(repo1edited)
		assert.True(t, *repo1editedOption.HasIssues)
		assert.Nil(t, repo1editedOption.ExternalTracker)
		assert.Equal(t, *repo1editedOption.InternalTracker, *repoEditOption.InternalTracker)
		assert.True(t, *repo1editedOption.HasWiki)
		assert.Nil(t, repo1editedOption.ExternalWiki)

		// Test editing repo1 to use external issue and wiki
		repoEditOption.ExternalTracker = &api.ExternalTracker{
			ExternalTrackerURL:    "http://www.somewebsite.com",
			ExternalTrackerFormat: "http://www.somewebsite.com/{user}/{repo}?issue={index}",
			ExternalTrackerStyle:  "alphanumeric",
		}
		repoEditOption.ExternalWiki = &api.ExternalWiki{
			ExternalWikiURL: "http://www.somewebsite.com",
		}
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		// check repo1 was written to database
		repo1edited = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		repo1editedOption = getRepoEditOptionFromRepo(repo1edited)
		assert.True(t, *repo1editedOption.HasIssues)
		assert.Equal(t, *repo1editedOption.ExternalTracker, *repoEditOption.ExternalTracker)
		assert.True(t, *repo1editedOption.HasWiki)
		assert.Equal(t, *repo1editedOption.ExternalWiki, *repoEditOption.ExternalWiki)
		assert.False(t, *repo1editedOption.HasCode)
		assert.False(t, *repo1editedOption.HasPackages)
		assert.False(t, *repo1editedOption.HasProjects)
		assert.False(t, *repo1editedOption.HasReleases)
		assert.False(t, *repo1editedOption.HasActions)

		repoEditOption.ExternalTracker.ExternalTrackerStyle = "regexp"
		repoEditOption.ExternalTracker.ExternalTrackerRegexpPattern = `(\d+)`
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		repo1edited = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		repo1editedOption = getRepoEditOptionFromRepo(repo1edited)
		assert.True(t, *repo1editedOption.HasIssues)
		assert.Equal(t, *repo1editedOption.ExternalTracker, *repoEditOption.ExternalTracker)

		// Do some tests with invalid URL for external tracker and wiki
		repoEditOption.ExternalTracker.ExternalTrackerURL = "htp://www.somewebsite.com"
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
		repoEditOption.ExternalTracker.ExternalTrackerURL = "http://www.somewebsite.com"
		repoEditOption.ExternalTracker.ExternalTrackerFormat = "http://www.somewebsite.com/{user/{repo}?issue={index}"
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
		repoEditOption.ExternalTracker.ExternalTrackerFormat = "http://www.somewebsite.com/{user}/{repo}?issue={index}"
		repoEditOption.ExternalWiki.ExternalWikiURL = "htp://www.somewebsite.com"
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusUnprocessableEntity)

		// Test small repo change through API with issue and wiki option not set; They shall not be touched.
		*repoEditOption.Description = "small change"
		repoEditOption.HasIssues = nil
		repoEditOption.ExternalTracker = nil
		repoEditOption.HasWiki = nil
		repoEditOption.ExternalWiki = nil
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		// check repo1 was written to database
		repo1edited = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		repo1editedOption = getRepoEditOptionFromRepo(repo1edited)
		assert.Equal(t, *repo1editedOption.Description, *repoEditOption.Description)
		assert.True(t, *repo1editedOption.HasIssues)
		assert.NotNil(t, *repo1editedOption.ExternalTracker)
		assert.True(t, *repo1editedOption.HasWiki)
		assert.NotNil(t, *repo1editedOption.ExternalWiki)
		assert.False(t, *repo1editedOption.HasCode)
		assert.False(t, *repo1editedOption.HasPackages)
		assert.False(t, *repo1editedOption.HasProjects)
		assert.False(t, *repo1editedOption.HasReleases)
		assert.False(t, *repo1editedOption.HasActions)

		// reset repo in db
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, *repoEditOption.Name), &origRepoEditOption).
			AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusOK)

		// Test editing a non-existing repo
		name := "repodoesnotexist"
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, name), &api.EditRepoOption{Name: &name}).
			AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusNotFound)

		// Test editing repo16 by user4 who does not have write access
		origRepoEditOption = getRepoEditOptionFromRepo(repo16)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo16.Name), &repoEditOption).
			AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusNotFound)

		// Tests a repo with no token given so will fail
		origRepoEditOption = getRepoEditOptionFromRepo(repo16)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo16.Name), &repoEditOption)
		_ = MakeRequest(t, req, http.StatusNotFound)

		// Test using access token for a private repo that the user of the token owns
		origRepoEditOption = getRepoEditOptionFromRepo(repo16)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo16.Name), &repoEditOption).
			AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusOK)
		// reset repo in db
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, *repoEditOption.Name), &origRepoEditOption).
			AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusOK)

		// Test making a repo public that is private
		repo16 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})
		assert.True(t, repo16.IsPrivate)
		repoEditOption = &api.EditRepoOption{
			Private: &bFalse,
		}
		url = fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo16.Name)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusOK)
		repo16 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})
		assert.False(t, repo16.IsPrivate)
		// Make it private again
		repoEditOption.Private = &bTrue
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption).
			AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusOK)

		// Test to change empty repo
		assert.False(t, repo15.IsArchived)
		url = fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo15.Name)
		req = NewRequestWithJSON(t, "PATCH", url, &api.EditRepoOption{
			Archived: &bTrue,
		}).AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusOK)
		repo15 = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 15})
		assert.True(t, repo15.IsArchived)
		req = NewRequestWithJSON(t, "PATCH", url, &api.EditRepoOption{
			Archived: &bFalse,
		}).AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusOK)

		// Test using org repo "org3/repo3" where user2 is a collaborator
		origRepoEditOption = getRepoEditOptionFromRepo(repo3)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", org3.Name, repo3.Name), &repoEditOption).
			AddTokenAuth(token2)
		MakeRequest(t, req, http.StatusOK)
		// reset repo in db
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", org3.Name, *repoEditOption.Name), &origRepoEditOption).
			AddTokenAuth(token2)
		_ = MakeRequest(t, req, http.StatusOK)

		// Test using org repo "org3/repo3" with no user token
		origRepoEditOption = getRepoEditOptionFromRepo(repo3)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", org3.Name, repo3.Name), &repoEditOption)
		MakeRequest(t, req, http.StatusNotFound)

		// Test using repo "user2/repo1" where user4 is a NOT collaborator
		origRepoEditOption = getRepoEditOptionFromRepo(repo1)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo1.Name), &repoEditOption).
			AddTokenAuth(token4)
		MakeRequest(t, req, http.StatusForbidden)
	})
}

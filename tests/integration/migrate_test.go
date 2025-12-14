// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/migrations"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateLocalPath(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user1"})

	old := setting.ImportLocalPaths
	setting.ImportLocalPaths = true

	basePath := t.TempDir()

	lowercasePath := filepath.Join(basePath, "lowercase")
	err := os.Mkdir(lowercasePath, 0o700)
	assert.NoError(t, err)

	err = migrations.IsMigrateURLAllowed(lowercasePath, adminUser)
	assert.NoError(t, err, "case lowercase path")

	mixedcasePath := filepath.Join(basePath, "mIxeDCaSe")
	err = os.Mkdir(mixedcasePath, 0o700)
	assert.NoError(t, err)

	err = migrations.IsMigrateURLAllowed(mixedcasePath, adminUser)
	assert.NoError(t, err, "case mixedcase path")

	setting.ImportLocalPaths = old
}

func TestMigrateGiteaForm(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		AllowLocalNetworks := setting.Migrations.AllowLocalNetworks
		setting.Migrations.AllowLocalNetworks = true
		AppVer := setting.AppVer
		// Gitea SDK (go-sdk) need to parse the AppVer from server response, so we must set it to a valid version string.
		setting.AppVer = "1.16.0"
		defer func() {
			setting.Migrations.AllowLocalNetworks = AllowLocalNetworks
			setting.AppVer = AppVer
			migrations.Init()
		}()
		assert.NoError(t, migrations.Init())

		ownerName := "user2"
		repoName := "repo1"
		repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: ownerName})
		session := loginUser(t, ownerName)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeReadMisc)

		// Step 0: verify the repo is available
		req := NewRequestf(t, "GET", "/%s/%s", ownerName, repoName)
		_ = session.MakeRequest(t, req, http.StatusOK)
		// Step 1: get the Gitea migration form
		req = NewRequestf(t, "GET", "/repo/migrate/?service_type=%d", structs.GiteaService)
		resp := session.MakeRequest(t, req, http.StatusOK)
		// Step 2: load the form
		htmlDoc := NewHTMLParser(t, resp.Body)
		form := htmlDoc.doc.Find(`form.ui.form[action^="/repo/migrate"]`)
		link, exists := form.Attr("action")
		assert.True(t, exists, "The template has changed")
		serviceInput, exists := form.Find(`input[name="service"]`).Attr("value")
		assert.True(t, exists)
		assert.Equal(t, fmt.Sprintf("%d", structs.GiteaService), serviceInput)
		// Step 4: submit the migration to only migrate issues
		migratedRepoName := "otherrepo"
		req = NewRequestWithValues(t, "POST", link, map[string]string{
			"_csrf":       htmlDoc.GetCSRF(),
			"service":     fmt.Sprintf("%d", structs.GiteaService),
			"clone_addr":  fmt.Sprintf("%s%s/%s", u, ownerName, repoName),
			"auth_token":  token,
			"issues":      "on",
			"repo_name":   migratedRepoName,
			"description": "",
			"uid":         strconv.FormatInt(repoOwner.ID, 10),
		})
		resp = session.MakeRequest(t, req, http.StatusSeeOther)
		// Step 5: a redirection displays the migrated repository
		loc := resp.Header().Get("Location")
		assert.Equal(t, fmt.Sprintf("/%s/%s", ownerName, migratedRepoName), loc)
		// Step 6: check the repo was created
		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: migratedRepoName})
	})
}

func Test_UpdateCommentsMigrationsByType(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	err := issues_model.UpdateCommentsMigrationsByType(t.Context(), structs.GithubService, "1", 1)
	assert.NoError(t, err)
}

func Test_MigrateFromGiteaToGitea(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)

	resp, err := http.Get("https://gitea.com/gitea")
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		t.Skipf("Can't reach https://gitea.com, skipping %s", t.Name())
	}
	resp.Body.Close()

	repoName := fmt.Sprintf("gitea-to-gitea-%d", time.Now().UnixNano())
	cloneAddr := "https://gitea.com/gitea/test_repo.git"

	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate", &structs.MigrateRepoOptions{
		CloneAddr:    cloneAddr,
		RepoOwnerID:  owner.ID,
		RepoName:     repoName,
		Service:      structs.GiteaService.Name(),
		Wiki:         true,
		Milestones:   true,
		Labels:       true,
		Issues:       true,
		PullRequests: true,
		Releases:     true,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	migratedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: repoName})
	assert.Equal(t, owner.ID, migratedRepo.OwnerID)
	assert.Equal(t, structs.GiteaService, migratedRepo.OriginalServiceType)
	assert.Equal(t, cloneAddr, migratedRepo.OriginalURL)

	issueCount := unittest.GetCount(t,
		&issues_model.Issue{RepoID: migratedRepo.ID},
		unittest.Cond("is_pull = ?", false),
	)
	assert.EqualValues(t, 7, issueCount)
	pullCount := unittest.GetCount(t,
		&issues_model.Issue{RepoID: migratedRepo.ID},
		unittest.Cond("is_pull = ?", true),
	)
	assert.EqualValues(t, 6, pullCount)

	issue4, err := issues_model.GetIssueWithAttrsByIndex(t.Context(), migratedRepo.ID, 4)
	require.NoError(t, err)
	assert.Equal(t, owner.ID, issue4.PosterID)
	assert.Equal(t, "Ghost", issue4.OriginalAuthor)
	assert.EqualValues(t, -1, issue4.OriginalAuthorID)
	assert.Equal(t, "what is this repo about?", issue4.Title)
	assert.True(t, issue4.IsClosed)
	assert.True(t, issue4.IsLocked)
	if assert.NotNil(t, issue4.Milestone) {
		assert.Equal(t, "V1", issue4.Milestone.Name)
	}
	labelNames := make([]string, 0, len(issue4.Labels))
	for _, label := range issue4.Labels {
		labelNames = append(labelNames, label.Name)
	}
	assert.Contains(t, labelNames, "Question")
	reactionTypes := make([]string, 0, len(issue4.Reactions))
	for _, reaction := range issue4.Reactions {
		reactionTypes = append(reactionTypes, reaction.Type)
	}
	assert.ElementsMatch(t, []string{"laugh"}, reactionTypes) // gitea's author is ghost which will be ignored when migrating reactions

	comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
		IssueID: issue4.ID,
		Type:    issues_model.CommentTypeComment,
	})
	require.NoError(t, err)
	require.Len(t, comments, 2)
	assert.Equal(t, owner.ID, comments[0].PosterID)
	assert.EqualValues(t, 689, comments[0].OriginalAuthorID)
	assert.Equal(t, "6543", comments[0].OriginalAuthor)
	assert.True(t, strings.Contains(comments[0].Content, "TESTSET for gitea2gitea"))
	assert.Equal(t, owner.ID, comments[1].PosterID)
	assert.Equal(t, "Ghost", comments[1].OriginalAuthor)
	assert.EqualValues(t, -1, comments[1].OriginalAuthorID)
	assert.Equal(t, "Oh!", strings.TrimSpace(comments[1].Content))

	pr12, err := issues_model.GetPullRequestByIndex(t.Context(), migratedRepo.ID, 12)
	require.NoError(t, err)
	assert.Equal(t, owner.ID, pr12.Issue.PosterID)
	assert.Equal(t, "6543", pr12.Issue.OriginalAuthor)
	assert.EqualValues(t, 689, pr12.Issue.OriginalAuthorID)
	assert.Equal(t, "Dont Touch", pr12.Issue.Title)
	assert.True(t, pr12.Issue.IsClosed)
	assert.True(t, pr12.HasMerged)
	assert.Equal(t, "827aa28a907853e5ddfa40c8f9bc52471a2685fd", pr12.MergedCommitID)
	assert.NoError(t, pr12.Issue.LoadMilestone(t.Context()))
	if assert.NotNil(t, pr12.Issue.Milestone) {
		assert.Equal(t, "V2 Finalize", pr12.Issue.Milestone.Name)
	}
	assert.Contains(t, pr12.Issue.Content, "dont touch")
}

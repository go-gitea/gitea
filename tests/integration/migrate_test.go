// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
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

// setupGiteaMockServer creates a mock HTTP server that replays API responses from fixture files.
// If a GITEA_TOKEN environment variable is set, the mock server proxies requests to the live
// gitea.com instance and saves the responses as fixture files for future test runs.
// Example: GITEA_TOKEN=your_token go test -run Test_MigrateFromGiteaToGitea
func setupGiteaMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	giteaToken := os.Getenv("GITEA_TOKEN")
	liveMode := giteaToken != ""

	// fast-import data creates deterministic commits (fixed author/committer/timestamps),
	// so the resulting SHAs are always the same across runs.
	fastImportData := `commit refs/heads/master
mark :1
author Test <test@test.com> 1000000000 +0000
committer Test <test@test.com> 1000000000 +0000
data 8
initial

commit refs/heads/master
mark :2
author Test <test@test.com> 1000000001 +0000
committer Test <test@test.com> 1000000001 +0000
data 7
second

from :1

commit refs/heads/6543-patch-1
mark :3
author Test <test@test.com> 1000000002 +0000
committer Test <test@test.com> 1000000002 +0000
data 6
patch

from :2

reset refs/tags/V1
from :1

reset refs/tags/v2-rc1
from :2

done
`
	// Fork adds one extra branch for the PR head (from master = 873987e)
	forkExtraData := `commit refs/heads/add-xkcd-2199
author Test <test@test.com> 1000000003 +0000
committer Test <test@test.com> 1000000003 +0000
data 5
xkcd

from 873987ea3e99c206bb0841266845098ee74d4ce9

done
`
	fastImport := func(dir, data string) {
		cmd := exec.Command("git", "-C", dir, "fast-import", "--date-format=raw", "--done")
		cmd.Stdin = strings.NewReader(data)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "fast-import failed: %s", out)
	}

	repoDir := t.TempDir()
	out, err := exec.Command("git", "init", "--bare", repoDir).CombinedOutput()
	require.NoError(t, err, "git init failed: %s", out)
	fastImport(repoDir, fastImportData)

	forkDir := t.TempDir()
	out, err = exec.Command("git", "clone", "--bare", repoDir, forkDir).CombinedOutput()
	require.NoError(t, err, "git clone failed: %s", out)
	fastImport(forkDir, forkExtraData)

	// Find git-http-backend
	execPathBytes, err := exec.Command("git", "--exec-path").Output()
	require.NoError(t, err)
	httpBackend := filepath.Join(strings.TrimSpace(string(execPathBytes)), "git-http-backend")

	_, callerFile, _, _ := runtime.Caller(0)
	fixtureDir := filepath.Join(filepath.Dir(callerFile), "_mock_data/Test_MigrateFromGiteaToGitea")
	return unittest.NewMockWebServer(t, "https://gitea.com",
		fixtureDir, liveMode,
		unittest.MockServerOptions{
			Routes: func(mux *http.ServeMux) {
				mux.HandleFunc("/gitea/test_repo.wiki.git/", func(w http.ResponseWriter, _ *http.Request) {
					http.Error(w, "wiki not found", http.StatusNotFound)
				})
				gitHandler := func(dir, prefix string) http.HandlerFunc {
					return func(w http.ResponseWriter, r *http.Request) {
						handler := &cgi.Handler{
							Path: httpBackend,
							Dir:  dir,
							Env: []string{
								"GIT_PROJECT_ROOT=" + filepath.Dir(dir),
								"GIT_HTTP_EXPORT_ALL=1",
							},
						}
						r.URL.Path = "/" + filepath.Base(dir) + strings.TrimPrefix(r.URL.Path, prefix)
						handler.ServeHTTP(w, r)
					}
				}
				mux.HandleFunc("/gitea/test_repo.git/", gitHandler(repoDir, "/gitea/test_repo.git"))
				mux.HandleFunc("/6543-forks/test_repo.git/", gitHandler(forkDir, "/6543-forks/test_repo.git"))
			},
		},
	)
}

func Test_MigrateFromGiteaToGitea(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	AllowLocalNetworks := setting.Migrations.AllowLocalNetworks
	setting.Migrations.AllowLocalNetworks = true
	defer func() {
		setting.Migrations.AllowLocalNetworks = AllowLocalNetworks
		migrations.Init()
	}()
	require.NoError(t, migrations.Init())

	mockServer := setupGiteaMockServer(t)

	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})
	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)

	repoName := "migrated-from-mock-gitea"
	cloneAddr := mockServer.URL + "/gitea/test_repo.git"

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
	assert.Equal(t, 7, issueCount)
	pullCount := unittest.GetCount(t,
		&issues_model.Issue{RepoID: migratedRepo.ID},
		unittest.Cond("is_pull = ?", true),
	)
	assert.Equal(t, 6, pullCount)

	issue4, err := issues_model.GetIssueWithAttrsByIndex(t.Context(), migratedRepo.ID, 4)
	require.NoError(t, err)
	assert.Equal(t, owner.ID, issue4.PosterID)
	assert.Equal(t, "Ghost", issue4.OriginalAuthor)
	assert.Equal(t, int64(-1), issue4.OriginalAuthorID)
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
	assert.Equal(t, int64(689), comments[0].OriginalAuthorID)
	assert.Equal(t, "6543", comments[0].OriginalAuthor)
	assert.Contains(t, comments[0].Content, "TESTSET for gitea2gitea")
	assert.Equal(t, owner.ID, comments[1].PosterID)
	assert.Equal(t, "Ghost", comments[1].OriginalAuthor)
	assert.Equal(t, int64(-1), comments[1].OriginalAuthorID)
	assert.Equal(t, "Oh!", strings.TrimSpace(comments[1].Content))

	pr12, err := issues_model.GetPullRequestByIndex(t.Context(), migratedRepo.ID, 12)
	require.NoError(t, err)
	assert.Equal(t, owner.ID, pr12.Issue.PosterID)
	assert.Equal(t, "6543", pr12.Issue.OriginalAuthor)
	assert.Equal(t, int64(689), pr12.Issue.OriginalAuthorID)
	assert.Equal(t, "Dont Touch", pr12.Issue.Title)
	assert.True(t, pr12.Issue.IsClosed)
	assert.True(t, pr12.HasMerged)
	assert.Equal(t, "827aa28a907853e5ddfa40c8f9bc52471a2685fd", pr12.MergedCommitID)
	assert.NoError(t, pr12.Issue.LoadMilestone(t.Context()))
	if assert.NotNil(t, pr12.Issue.Milestone) {
		assert.Equal(t, "V2 Finalize", pr12.Issue.Milestone.Name)
	}
	assert.Contains(t, pr12.Issue.Content, "dont touch")

	pr8, err := issues_model.GetPullRequestByIndex(t.Context(), migratedRepo.ID, 8)
	require.NoError(t, err)
	assert.Equal(t, owner.ID, pr8.Issue.PosterID)
	assert.Equal(t, "6543", pr8.Issue.OriginalAuthor)
	assert.Equal(t, int64(689), pr8.Issue.OriginalAuthorID)
	assert.Equal(t, "add garbage for close pull", pr8.Issue.Title)
	assert.True(t, pr8.Issue.IsClosed)
	assert.False(t, pr8.HasMerged)
	assert.Contains(t, pr8.Issue.Content, "well you'll see")

	pr13, err := issues_model.GetPullRequestByIndex(t.Context(), migratedRepo.ID, 13)
	require.NoError(t, err)
	assert.Equal(t, owner.ID, pr13.Issue.PosterID)
	assert.Equal(t, "6543", pr13.Issue.OriginalAuthor)
	assert.Equal(t, int64(689), pr13.Issue.OriginalAuthorID)
	assert.Equal(t, "extend", pr13.Issue.Title)
	assert.False(t, pr13.Issue.IsClosed)
	assert.False(t, pr13.HasMerged)
	assert.True(t, pr13.Issue.IsLocked)

	gitRepo, err := gitrepo.OpenRepository(t.Context(), migratedRepo)
	require.NoError(t, err)
	defer gitRepo.Close()

	branches, _, err := gitRepo.GetBranchNames(0, 0)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"6543-patch-1", "master", "6543-forks/add-xkcd-2199"}, branches) // last branch comes from the pull request

	branchNames, err := git_model.FindBranchNames(t.Context(), git_model.FindBranchOptions{
		RepoID: migratedRepo.ID,
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"6543-patch-1", "master", "6543-forks/add-xkcd-2199"}, branchNames)

	tags, _, err := gitRepo.GetTagInfos(0, 0)
	require.NoError(t, err)
	tagNames := make([]string, 0, len(tags))
	for _, tag := range tags {
		tagNames = append(tagNames, tag.Name)
	}
	assert.ElementsMatch(t, []string{"V1", "v2-rc1"}, tagNames)

	releases, err := db.Find[repo_model.Release](t.Context(), repo_model.FindReleasesOptions{
		RepoID:        migratedRepo.ID,
		IncludeDrafts: true,
		IncludeTags:   false,
	})
	require.NoError(t, err)
	require.Len(t, releases, 2)

	releaseMap := make(map[string]*repo_model.Release, len(releases))
	for _, rel := range releases {
		releaseMap[rel.TagName] = rel
		assert.Equal(t, owner.ID, rel.PublisherID)
		assert.Equal(t, "6543", rel.OriginalAuthor)
		assert.Equal(t, int64(689), rel.OriginalAuthorID)
		assert.False(t, rel.IsDraft)
	}

	require.Contains(t, releaseMap, "v2-rc1")
	v2Release := releaseMap["v2-rc1"]
	assert.Equal(t, "Second Release", v2Release.Title)
	assert.True(t, v2Release.IsPrerelease)
	assert.Contains(t, v2Release.Note, "this repo has:")

	require.Contains(t, releaseMap, "V1")
	v1Release := releaseMap["V1"]
	assert.Equal(t, "First Release", v1Release.Title)
	assert.False(t, v1Release.IsPrerelease)
	assert.Equal(t, "as title", strings.TrimSpace(v1Release.Note))
}

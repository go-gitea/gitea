// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUserReposNotLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	req := NewRequestf(t, "GET", "/api/v1/users/%s/repos", user.Name)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiRepos []api.Repository
	DecodeJSON(t, resp, &apiRepos)
	expectedLen := unittest.GetCount(t, repo_model.Repository{OwnerID: user.ID},
		unittest.Cond("is_private = ?", false))
	assert.Len(t, apiRepos, expectedLen)
	for _, repo := range apiRepos {
		assert.EqualValues(t, user.ID, repo.Owner.ID)
		assert.False(t, repo.Private)
	}
}

func TestAPISearchRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const keyword = "test"

	req := NewRequestf(t, "GET", "/api/v1/repos/search?q=%s", keyword)
	resp := MakeRequest(t, req, http.StatusOK)

	var body api.SearchResults
	DecodeJSON(t, resp, &body)
	assert.NotEmpty(t, body.Data)
	for _, repo := range body.Data {
		assert.Contains(t, repo.Name, keyword)
		assert.False(t, repo.Private)
	}

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 16})
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 18})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 20})
	orgUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 17})

	oldAPIDefaultNum := setting.API.DefaultPagingNum
	defer func() {
		setting.API.DefaultPagingNum = oldAPIDefaultNum
	}()
	setting.API.DefaultPagingNum = 10

	// Map of expected results, where key is user for login
	type expectedResults map[*user_model.User]struct {
		count           int
		repoOwnerID     int64
		repoName        string
		includesPrivate bool
	}

	testCases := []struct {
		name, requestURL string
		expectedResults
	}{
		{
			name: "RepositoriesMax50", requestURL: "/api/v1/repos/search?limit=50&private=false", expectedResults: expectedResults{
				nil:   {count: 32},
				user:  {count: 32},
				user2: {count: 32},
			},
		},
		{
			name: "RepositoriesMax10", requestURL: "/api/v1/repos/search?limit=10&private=false", expectedResults: expectedResults{
				nil:   {count: 10},
				user:  {count: 10},
				user2: {count: 10},
			},
		},
		{
			name: "RepositoriesDefault", requestURL: "/api/v1/repos/search?default&private=false", expectedResults: expectedResults{
				nil:   {count: 10},
				user:  {count: 10},
				user2: {count: 10},
			},
		},
		{
			name: "RepositoriesByName", requestURL: fmt.Sprintf("/api/v1/repos/search?q=%s&private=false", "big_test_"), expectedResults: expectedResults{
				nil:   {count: 7, repoName: "big_test_"},
				user:  {count: 7, repoName: "big_test_"},
				user2: {count: 7, repoName: "big_test_"},
			},
		},
		{
			name: "RepositoriesByName", requestURL: fmt.Sprintf("/api/v1/repos/search?q=%s&private=false", "user2/big_test_"), expectedResults: expectedResults{
				user2: {count: 2, repoName: "big_test_"},
			},
		},
		{
			name: "RepositoriesAccessibleAndRelatedToUser", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user.ID), expectedResults: expectedResults{
				nil:   {count: 5},
				user:  {count: 9, includesPrivate: true},
				user2: {count: 6, includesPrivate: true},
			},
		},
		{
			name: "RepositoriesAccessibleAndRelatedToUser2", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user2.ID), expectedResults: expectedResults{
				nil:   {count: 1},
				user:  {count: 2, includesPrivate: true},
				user2: {count: 2, includesPrivate: true},
				user4: {count: 1},
			},
		},
		{
			name: "RepositoriesAccessibleAndRelatedToUser3", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user3.ID), expectedResults: expectedResults{
				nil:   {count: 1},
				user:  {count: 4, includesPrivate: true},
				user2: {count: 3, includesPrivate: true},
				user3: {count: 4, includesPrivate: true},
			},
		},
		{
			name: "RepositoriesOwnedByOrganization", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", orgUser.ID), expectedResults: expectedResults{
				nil:   {count: 1, repoOwnerID: orgUser.ID},
				user:  {count: 2, repoOwnerID: orgUser.ID, includesPrivate: true},
				user2: {count: 1, repoOwnerID: orgUser.ID},
			},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser4", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user4.ID), expectedResults: expectedResults{
			nil:   {count: 3},
			user:  {count: 4, includesPrivate: true},
			user4: {count: 7, includesPrivate: true},
		}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeSource", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s", user4.ID, "source"), expectedResults: expectedResults{
			nil:   {count: 0},
			user:  {count: 1, includesPrivate: true},
			user4: {count: 1, includesPrivate: true},
		}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeFork", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s", user4.ID, "fork"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true},
		}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeFork/Exclusive", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s&exclusive=1", user4.ID, "fork"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true},
		}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeMirror", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s", user4.ID, "mirror"), expectedResults: expectedResults{
			nil:   {count: 2},
			user:  {count: 2},
			user4: {count: 4, includesPrivate: true},
		}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeMirror/Exclusive", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s&exclusive=1", user4.ID, "mirror"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true},
		}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeCollaborative", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d&mode=%s", user4.ID, "collaborative"), expectedResults: expectedResults{
			nil:   {count: 0},
			user:  {count: 1, includesPrivate: true},
			user4: {count: 1, includesPrivate: true},
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for userToLogin, expected := range testCase.expectedResults {
				var testName string
				var userID int64
				var token string
				if userToLogin != nil && userToLogin.ID > 0 {
					testName = fmt.Sprintf("LoggedUser%d", userToLogin.ID)
					session := loginUser(t, userToLogin.Name)
					token = getTokenForLoggedInUser(t, session)
					userID = userToLogin.ID
				} else {
					testName = "AnonymousUser"
					_ = emptyTestSession(t)
				}

				t.Run(testName, func(t *testing.T) {
					request := NewRequest(t, "GET", testCase.requestURL+"&token="+token)
					response := MakeRequest(t, request, http.StatusOK)

					var body api.SearchResults
					DecodeJSON(t, response, &body)

					repoNames := make([]string, 0, len(body.Data))
					for _, repo := range body.Data {
						repoNames = append(repoNames, fmt.Sprintf("%d:%s:%t", repo.ID, repo.FullName, repo.Private))
					}
					assert.Len(t, repoNames, expected.count)
					for _, repo := range body.Data {
						r := getRepo(t, repo.ID)
						hasAccess, err := access_model.HasAccess(db.DefaultContext, userID, r)
						assert.NoError(t, err, "Error when checking if User: %d has access to %s: %v", userID, repo.FullName, err)
						assert.True(t, hasAccess, "User: %d does not have access to %s", userID, repo.FullName)

						assert.NotEmpty(t, repo.Name)
						assert.Equal(t, repo.Name, r.Name)

						if len(expected.repoName) > 0 {
							assert.Contains(t, repo.Name, expected.repoName)
						}

						if expected.repoOwnerID > 0 {
							assert.Equal(t, expected.repoOwnerID, repo.Owner.ID)
						}

						if !expected.includesPrivate {
							assert.False(t, repo.Private, "User: %d not expecting private repository: %s", userID, repo.FullName)
						}
					}
				})
			}
		})
	}
}

var repoCache = make(map[int64]*repo_model.Repository)

func getRepo(t *testing.T, repoID int64) *repo_model.Repository {
	if _, ok := repoCache[repoID]; !ok {
		repoCache[repoID] = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
	}
	return repoCache[repoID]
}

func TestAPIViewRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	var repo api.Repository

	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &repo)
	assert.EqualValues(t, 1, repo.ID)
	assert.EqualValues(t, "repo1", repo.Name)
	assert.EqualValues(t, 2, repo.Releases)
	assert.EqualValues(t, 1, repo.OpenIssues)
	assert.EqualValues(t, 3, repo.OpenPulls)

	req = NewRequest(t, "GET", "/api/v1/repos/user12/repo10")
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &repo)
	assert.EqualValues(t, 10, repo.ID)
	assert.EqualValues(t, "repo10", repo.Name)
	assert.EqualValues(t, 1, repo.OpenPulls)
	assert.EqualValues(t, 1, repo.Forks)

	req = NewRequest(t, "GET", "/api/v1/repos/user5/repo4")
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &repo)
	assert.EqualValues(t, 4, repo.ID)
	assert.EqualValues(t, "repo4", repo.Name)
	assert.EqualValues(t, 1, repo.Stars)
}

func TestAPIOrgRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	// User3 is an Org. Check their repos.
	sourceOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})

	expectedResults := map[*user_model.User]struct {
		count           int
		includesPrivate bool
	}{
		user:  {count: 1},
		user:  {count: 3, includesPrivate: true},
		user2: {count: 3, includesPrivate: true},
		user3: {count: 1},
	}

	for userToLogin, expected := range expectedResults {
		testName := fmt.Sprintf("LoggedUser%d", userToLogin.ID)
		session := loginUser(t, userToLogin.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrg)

		t.Run(testName, func(t *testing.T) {
			req := NewRequestf(t, "GET", "/api/v1/orgs/%s/repos?token="+token, sourceOrg.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			var apiRepos []*api.Repository
			DecodeJSON(t, resp, &apiRepos)
			assert.Len(t, apiRepos, expected.count)
			for _, repo := range apiRepos {
				if !expected.includesPrivate {
					assert.False(t, repo.Private)
				}
			}
		})
	}
}

func TestAPIGetRepoByIDUnauthorized(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	req := NewRequestf(t, "GET", "/api/v1/repositories/2?token="+token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIRepoMigrate(t *testing.T) {
	testCases := []struct {
		ctxUserID, userID  int64
		cloneURL, repoName string
		expectedStatus     int
	}{
		{ctxUserID: 1, userID: 2, cloneURL: "https://github.com/go-gitea/test_repo.git", repoName: "git-admin", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, userID: 2, cloneURL: "https://github.com/go-gitea/test_repo.git", repoName: "git-own", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, userID: 1, cloneURL: "https://github.com/go-gitea/test_repo.git", repoName: "git-bad", expectedStatus: http.StatusForbidden},
		{ctxUserID: 2, userID: 3, cloneURL: "https://github.com/go-gitea/test_repo.git", repoName: "git-org", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, userID: 6, cloneURL: "https://github.com/go-gitea/test_repo.git", repoName: "git-bad-org", expectedStatus: http.StatusForbidden},
		{ctxUserID: 2, userID: 3, cloneURL: "https://localhost:3000/user/test_repo.git", repoName: "private-ip", expectedStatus: http.StatusUnprocessableEntity},
		{ctxUserID: 2, userID: 3, cloneURL: "https://10.0.0.1/user/test_repo.git", repoName: "private-ip", expectedStatus: http.StatusUnprocessableEntity},
	}

	defer tests.PrepareTestEnv(t)()
	for _, testCase := range testCases {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: testCase.ctxUserID})
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate?token="+token, &api.MigrateRepoOptions{
			CloneAddr:   testCase.cloneURL,
			RepoOwnerID: testCase.userID,
			RepoName:    testCase.repoName,
		})
		resp := MakeRequest(t, req, NoExpectedStatus)
		if resp.Code == http.StatusUnprocessableEntity {
			respJSON := map[string]string{}
			DecodeJSON(t, resp, &respJSON)
			switch respJSON["message"] {
			case "Remote visit addressed rate limitation.":
				t.Log("test hit github rate limitation")
			case "You can not import from disallowed hosts.":
				assert.EqualValues(t, "private-ip", testCase.repoName)
			default:
				assert.Failf(t, "unexpected error '%v' on url '%s'", respJSON["message"], testCase.cloneURL)
			}
		} else {
			assert.EqualValues(t, testCase.expectedStatus, resp.Code)
		}
	}
}

func TestAPIRepoMigrateConflict(t *testing.T) {
	onGiteaRun(t, testAPIRepoMigrateConflict)
}

func testAPIRepoMigrateConflict(t *testing.T, u *url.URL) {
	username := "user2"
	baseAPITestContext := NewAPITestContext(t, username, "repo1", auth_model.AccessTokenScopeRepo)

	u.Path = baseAPITestContext.GitPath()

	t.Run("Existing", func(t *testing.T) {
		httpContext := baseAPITestContext

		httpContext.Reponame = "repo-tmp-17"
		t.Run("CreateRepo", doAPICreateRepository(httpContext, false))

		user, err := user_model.GetUserByName(db.DefaultContext, httpContext.Username)
		assert.NoError(t, err)
		userID := user.ID

		cloneURL := "https://github.com/go-gitea/test_repo.git"

		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate?token="+httpContext.Token,
			&api.MigrateRepoOptions{
				CloneAddr:   cloneURL,
				RepoOwnerID: userID,
				RepoName:    httpContext.Reponame,
			})
		resp := httpContext.Session.MakeRequest(t, req, http.StatusConflict)
		respJSON := map[string]string{}
		DecodeJSON(t, resp, &respJSON)
		assert.Equal(t, "The repository with the same name already exists.", respJSON["message"])
	})
}

// mirror-sync must fail with "400 (Bad Request)" when an attempt is made to
// sync a non-mirror repository.
func TestAPIMirrorSyncNonMirrorRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)

	var repo api.Repository
	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &repo)
	assert.False(t, repo.Mirror)

	req = NewRequestf(t, "POST", "/api/v1/repos/user2/repo1/mirror-sync?token=%s", token)
	resp = MakeRequest(t, req, http.StatusBadRequest)
	errRespJSON := map[string]string{}
	DecodeJSON(t, resp, &errRespJSON)
	assert.Equal(t, "Repository is not a mirror", errRespJSON["message"])
}

func TestAPIOrgRepoCreate(t *testing.T) {
	testCases := []struct {
		ctxUserID         int64
		orgName, repoName string
		expectedStatus    int
	}{
		{ctxUserID: 1, orgName: "user3", repoName: "repo-admin", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, orgName: "user3", repoName: "repo-own", expectedStatus: http.StatusCreated},
		{ctxUserID: 2, orgName: "user6", repoName: "repo-bad-org", expectedStatus: http.StatusForbidden},
		{ctxUserID: 28, orgName: "user3", repoName: "repo-creator", expectedStatus: http.StatusCreated},
		{ctxUserID: 28, orgName: "user6", repoName: "repo-not-creator", expectedStatus: http.StatusForbidden},
	}

	defer tests.PrepareTestEnv(t)()
	for _, testCase := range testCases {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: testCase.ctxUserID})
		session := loginUser(t, user.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAdminOrg)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/org/%s/repos?token="+token, testCase.orgName), &api.CreateRepoOption{
			Name: testCase.repoName,
		})
		MakeRequest(t, req, testCase.expectedStatus)
	}
}

func TestAPIRepoCreateConflict(t *testing.T) {
	onGiteaRun(t, testAPIRepoCreateConflict)
}

func testAPIRepoCreateConflict(t *testing.T, u *url.URL) {
	username := "user2"
	baseAPITestContext := NewAPITestContext(t, username, "repo1", auth_model.AccessTokenScopeRepo)

	u.Path = baseAPITestContext.GitPath()

	t.Run("Existing", func(t *testing.T) {
		httpContext := baseAPITestContext

		httpContext.Reponame = "repo-tmp-17"
		t.Run("CreateRepo", doAPICreateRepository(httpContext, false))

		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos?token="+httpContext.Token,
			&api.CreateRepoOption{
				Name: httpContext.Reponame,
			})
		resp := httpContext.Session.MakeRequest(t, req, http.StatusConflict)
		respJSON := map[string]string{}
		DecodeJSON(t, resp, &respJSON)
		assert.Equal(t, "The repository with the same name already exists.", respJSON["message"])
	})
}

func TestAPIRepoTransfer(t *testing.T) {
	testCases := []struct {
		ctxUserID      int64
		newOwner       string
		teams          *[]int64
		expectedStatus int
	}{
		// Disclaimer for test story: "user1" is an admin, "user2" is normal user and part of in owner team of org "user3"
		// Transfer to a user with teams in another org should fail
		{ctxUserID: 1, newOwner: "user3", teams: &[]int64{5}, expectedStatus: http.StatusForbidden},
		// Transfer to a user with non-existent team IDs should fail
		{ctxUserID: 1, newOwner: "user2", teams: &[]int64{2}, expectedStatus: http.StatusUnprocessableEntity},
		// Transfer should go through
		{ctxUserID: 1, newOwner: "user3", teams: &[]int64{2}, expectedStatus: http.StatusAccepted},
		// Let user transfer it back to himself
		{ctxUserID: 2, newOwner: "user2", expectedStatus: http.StatusAccepted},
		// And revert transfer
		{ctxUserID: 2, newOwner: "user3", teams: &[]int64{2}, expectedStatus: http.StatusAccepted},
		// Cannot start transfer to an existing repo
		{ctxUserID: 2, newOwner: "user3", teams: nil, expectedStatus: http.StatusUnprocessableEntity},
		// Start transfer, repo is now in pending transfer mode
		{ctxUserID: 2, newOwner: "user6", teams: nil, expectedStatus: http.StatusCreated},
	}

	defer tests.PrepareTestEnv(t)()

	// create repo to move
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	repoName := "moveME"
	apiRepo := new(api.Repository)
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/user/repos?token=%s", token), &api.CreateRepoOption{
		Name:        repoName,
		Description: "repo move around",
		Private:     false,
		Readme:      "Default",
		AutoInit:    true,
	})
	resp := MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, apiRepo)

	// start testing
	for _, testCase := range testCases {
		user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: testCase.ctxUserID})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
		session = loginUser(t, user.Name)
		token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer?token=%s", repo.OwnerName, repo.Name, token), &api.TransferRepoOption{
			NewOwner: testCase.newOwner,
			TeamIDs:  testCase.teams,
		})
		MakeRequest(t, req, testCase.expectedStatus)
	}

	// cleanup
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
	_ = models.DeleteRepository(user, repo.OwnerID, repo.ID)
}

func transfer(t *testing.T) *repo_model.Repository {
	// create repo to move
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	repoName := "moveME"
	apiRepo := new(api.Repository)
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/user/repos?token=%s", token), &api.CreateRepoOption{
		Name:        repoName,
		Description: "repo move around",
		Private:     false,
		Readme:      "Default",
		AutoInit:    true,
	})

	resp := MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, apiRepo)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: apiRepo.ID})
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer?token=%s", repo.OwnerName, repo.Name, token), &api.TransferRepoOption{
		NewOwner: "user4",
	})
	MakeRequest(t, req, http.StatusCreated)

	return repo
}

func TestAPIAcceptTransfer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := transfer(t)

	// try to accept with not authorized user
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer/reject?token=%s", repo.OwnerName, repo.Name, token))
	MakeRequest(t, req, http.StatusForbidden)

	// try to accept repo that's not marked as transferred
	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer/accept?token=%s", "user2", "repo1", token))
	MakeRequest(t, req, http.StatusNotFound)

	// accept transfer
	session = loginUser(t, "user4")
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)

	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer/accept?token=%s", repo.OwnerName, repo.Name, token))
	resp := MakeRequest(t, req, http.StatusAccepted)
	apiRepo := new(api.Repository)
	DecodeJSON(t, resp, apiRepo)
	assert.Equal(t, "user4", apiRepo.Owner.UserName)
}

func TestAPIRejectTransfer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := transfer(t)

	// try to reject with not authorized user
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer/reject?token=%s", repo.OwnerName, repo.Name, token))
	MakeRequest(t, req, http.StatusForbidden)

	// try to reject repo that's not marked as transferred
	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer/reject?token=%s", "user2", "repo1", token))
	MakeRequest(t, req, http.StatusNotFound)

	// reject transfer
	session = loginUser(t, "user4")
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)

	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/transfer/reject?token=%s", repo.OwnerName, repo.Name, token))
	resp := MakeRequest(t, req, http.StatusOK)
	apiRepo := new(api.Repository)
	DecodeJSON(t, resp, apiRepo)
	assert.Equal(t, "user2", apiRepo.Owner.UserName)
}

func TestAPIGenerateRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)

	templateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 44})

	// user
	repo := new(api.Repository)
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/generate?token=%s", templateRepo.OwnerName, templateRepo.Name, token), &api.GenerateRepoOption{
		Owner:       user.Name,
		Name:        "new-repo",
		Description: "test generate repo",
		Private:     false,
		GitContent:  true,
	})
	resp := MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, repo)

	assert.Equal(t, "new-repo", repo.Name)

	// org
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/generate?token=%s", templateRepo.OwnerName, templateRepo.Name, token), &api.GenerateRepoOption{
		Owner:       "user3",
		Name:        "new-repo",
		Description: "test generate repo",
		Private:     false,
		GitContent:  true,
	})
	resp = MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, repo)

	assert.Equal(t, "new-repo", repo.Name)
}

func TestAPIRepoGetReviewers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/reviewers?token=%s", user.Name, repo.Name, token)
	resp := MakeRequest(t, req, http.StatusOK)
	var reviewers []*api.User
	DecodeJSON(t, resp, &reviewers)
	assert.Len(t, reviewers, 4)
}

func TestAPIRepoGetAssignees(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeRepo)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/assignees?token=%s", user.Name, repo.Name, token)
	resp := MakeRequest(t, req, http.StatusOK)
	var assignees []*api.User
	DecodeJSON(t, resp, &assignees)
	assert.Len(t, assignees, 1)
}

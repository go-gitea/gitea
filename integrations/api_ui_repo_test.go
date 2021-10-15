// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestAPIUISearchRepo(t *testing.T) {
	defer prepareTestEnv(t)()
	const keyword = "test"

	req := NewRequestf(t, "GET", "/api/ui/repos/search?q=%s", keyword)
	resp := MakeRequest(t, req, http.StatusOK)

	var body api.SearchResults
	DecodeJSON(t, resp, &body)
	assert.NotEmpty(t, body.Data)
	for _, repo := range body.Data {
		assert.Contains(t, repo.Name, keyword)
		assert.False(t, repo.Private)
	}

	user := db.AssertExistsAndLoadBean(t, &models.User{ID: 15}).(*models.User)
	user2 := db.AssertExistsAndLoadBean(t, &models.User{ID: 16}).(*models.User)
	user3 := db.AssertExistsAndLoadBean(t, &models.User{ID: 18}).(*models.User)
	user4 := db.AssertExistsAndLoadBean(t, &models.User{ID: 20}).(*models.User)
	orgUser := db.AssertExistsAndLoadBean(t, &models.User{ID: 17}).(*models.User)

	oldAPIDefaultNum := setting.API.DefaultPagingNum
	defer func() {
		setting.API.DefaultPagingNum = oldAPIDefaultNum
	}()
	setting.API.DefaultPagingNum = 10

	// Map of expected results, where key is user for login
	type expectedResults map[*models.User]struct {
		count           int
		repoOwnerID     int64
		repoName        string
		includesPrivate bool
	}

	testCases := []struct {
		name, requestURL string
		expectedResults
	}{
		{name: "RepositoriesMax50", requestURL: "/api/ui/repos/search?limit=50&private=false", expectedResults: expectedResults{
			nil:   {count: 30},
			user:  {count: 30},
			user2: {count: 30}},
		},
		{name: "RepositoriesMax10", requestURL: "/api/ui/repos/search?limit=10&private=false", expectedResults: expectedResults{
			nil:   {count: 10},
			user:  {count: 10},
			user2: {count: 10}},
		},
		{name: "RepositoriesDefault", requestURL: "/api/ui/repos/search?default&private=false", expectedResults: expectedResults{
			nil:   {count: 10},
			user:  {count: 10},
			user2: {count: 10}},
		},
		{name: "RepositoriesByName", requestURL: fmt.Sprintf("/api/ui/repos/search?q=%s&private=false", "big_test_"), expectedResults: expectedResults{
			nil:   {count: 7, repoName: "big_test_"},
			user:  {count: 7, repoName: "big_test_"},
			user2: {count: 7, repoName: "big_test_"}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d", user.ID), expectedResults: expectedResults{
			nil:   {count: 5},
			user:  {count: 9, includesPrivate: true},
			user2: {count: 6, includesPrivate: true}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser2", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d", user2.ID), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 2, includesPrivate: true},
			user2: {count: 2, includesPrivate: true},
			user4: {count: 1}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser3", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d", user3.ID), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 4, includesPrivate: true},
			user2: {count: 3, includesPrivate: true},
			user3: {count: 4, includesPrivate: true}},
		},
		{name: "RepositoriesOwnedByOrganization", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d", orgUser.ID), expectedResults: expectedResults{
			nil:   {count: 1, repoOwnerID: orgUser.ID},
			user:  {count: 2, repoOwnerID: orgUser.ID, includesPrivate: true},
			user2: {count: 1, repoOwnerID: orgUser.ID}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser4", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d", user4.ID), expectedResults: expectedResults{
			nil:   {count: 3},
			user:  {count: 4, includesPrivate: true},
			user4: {count: 7, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeSource", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d&mode=%s", user4.ID, "source"), expectedResults: expectedResults{
			nil:   {count: 0},
			user:  {count: 1, includesPrivate: true},
			user4: {count: 1, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeFork", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d&mode=%s", user4.ID, "fork"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeFork/Exclusive", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d&mode=%s&exclusive=1", user4.ID, "fork"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeMirror", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d&mode=%s", user4.ID, "mirror"), expectedResults: expectedResults{
			nil:   {count: 2},
			user:  {count: 2},
			user4: {count: 4, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeMirror/Exclusive", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d&mode=%s&exclusive=1", user4.ID, "mirror"), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user4: {count: 2, includesPrivate: true}}},
		{name: "RepositoriesAccessibleAndRelatedToUser4/SearchModeCollaborative", requestURL: fmt.Sprintf("/api/ui/repos/search?uid=%d&mode=%s", user4.ID, "collaborative"), expectedResults: expectedResults{
			nil:   {count: 0},
			user:  {count: 1, includesPrivate: true},
			user4: {count: 1, includesPrivate: true}}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for userToLogin, expected := range testCase.expectedResults {
				var session *TestSession
				var testName string
				var userID int64
				if userToLogin != nil && userToLogin.ID > 0 {
					testName = fmt.Sprintf("LoggedUser%d", userToLogin.ID)
					session = loginUser(t, userToLogin.Name)
					userID = userToLogin.ID
				} else {
					testName = "AnonymousUser"
					session = emptyTestSession(t)
				}

				t.Run(testName, func(t *testing.T) {
					request := NewRequest(t, "GET", testCase.requestURL)
					response := session.MakeRequest(t, request, http.StatusOK)

					var body api.SearchResults
					DecodeJSON(t, response, &body)

					repoNames := make([]string, 0, len(body.Data))
					for _, repo := range body.Data {
						repoNames = append(repoNames, fmt.Sprintf("%d:%s:%t", repo.ID, repo.FullName, repo.Private))
					}
					assert.Len(t, repoNames, expected.count)
					for _, repo := range body.Data {
						r := getRepo(t, repo.ID)
						hasAccess, err := models.HasAccess(userID, r)
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

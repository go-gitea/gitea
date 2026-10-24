// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	activities_model "gitea.dev/models/activities"
	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

const (
	privateActivityTestAdmin = "user1"
	privateActivityTestUser  = "user2"
)

// org3 is an organization so it is not usable here
const privateActivityTestOtherUser = "user4"

// activity helpers

func testPrivateActivityDoSomethingForActionEntries(t *testing.T) {
	repoBefore := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repoBefore.OwnerID})

	session := loginUser(t, privateActivityTestUser)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues?state=all", owner.Name, repoBefore.Name)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Body:  "test",
		Title: "test",
	}).AddTokenAuth(token)
	session.MakeRequest(t, req, http.StatusCreated)
}

// private activity helpers

func testPrivateActivityHelperEnablePrivateActivity(t *testing.T) {
	session := loginUser(t, privateActivityTestUser)
	req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"name":                  privateActivityTestUser,
		"email":                 privateActivityTestUser + "@example.com",
		"language":              "en-US",
		"keep_activity_private": "1",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
}

func testPrivateActivityHelperHasVisibleActivitiesInHTMLDoc(htmlDoc *HTMLDoc) bool {
	return htmlDoc.doc.Find("#activity-feed").Find(".item").Length() > 0
}

func testPrivateActivityHelperHasVisibleActivitiesFromSession(t *testing.T, session *TestSession) bool {
	req := NewRequestf(t, "GET", "/%s?tab=activity", privateActivityTestUser)
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleActivitiesInHTMLDoc(htmlDoc)
}

func testPrivateActivityHelperHasVisibleActivitiesFromPublic(t *testing.T) bool {
	req := NewRequestf(t, "GET", "/%s?tab=activity", privateActivityTestUser)
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleActivitiesInHTMLDoc(htmlDoc)
}

// heatmap UI helpers

func testPrivateActivityHelperHasVisibleHeatmapInHTMLDoc(htmlDoc *HTMLDoc) bool {
	return htmlDoc.doc.Find("#user-heatmap").Length() > 0
}

func testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t *testing.T, session *TestSession) bool {
	req := NewRequestf(t, "GET", "/%s?tab=activity", privateActivityTestUser)
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleHeatmapInHTMLDoc(htmlDoc)
}

func testPrivateActivityHelperHasVisibleDashboardHeatmapFromSession(t *testing.T, session *TestSession) bool {
	req := NewRequest(t, "GET", "/")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleHeatmapInHTMLDoc(htmlDoc)
}

func testPrivateActivityHelperHasVisibleHeatmapFromPublic(t *testing.T) bool {
	req := NewRequestf(t, "GET", "/%s?tab=activity", privateActivityTestUser)
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleHeatmapInHTMLDoc(htmlDoc)
}

// heatmap API helpers

func testPrivateActivityHelperHasHeatmapContentFromPublic(t *testing.T) bool {
	req := NewRequestf(t, "GET", "/api/v1/users/%s/heatmap", privateActivityTestUser)
	resp := MakeRequest(t, req, http.StatusOK)

	items := DecodeJSON(t, resp, []*activities_model.UserHeatmapData{})

	return len(items) != 0
}

func testPrivateActivityHelperHasHeatmapContentFromSession(t *testing.T, session *TestSession) bool {
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeAll)

	req := NewRequestf(t, "GET", "/api/v1/users/%s/heatmap", privateActivityTestUser).
		AddTokenAuth(token)
	resp := session.MakeRequest(t, req, http.StatusOK)

	items := DecodeJSON(t, resp, []*activities_model.UserHeatmapData{})

	return len(items) != 0
}

func TestPrivateActivityVisibility(t *testing.T) {
	for _, test := range []struct {
		name    string
		private bool
		user    string
		visible bool
	}{
		// check activity visibility if the visibility is enabled
		{"NoVisibleForPublic", false, "", true},
		{"NoVisibleForUserItself", false, privateActivityTestUser, true},
		{"NoVisibleForOtherUser", false, privateActivityTestOtherUser, true},
		{"NoVisibleForAdmin", false, privateActivityTestAdmin, true},
		// check activity visibility if the visibility is disabled
		{"YesInvisibleForPublic", true, "", false},
		{"YesVisibleForUserItself", true, privateActivityTestUser, true},
		{"YesInvisibleForOtherUser", true, privateActivityTestOtherUser, false},
		{"YesVisibleForAdmin", true, privateActivityTestAdmin, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()
			testPrivateActivityDoSomethingForActionEntries(t)
			if test.private {
				testPrivateActivityHelperEnablePrivateActivity(t)
			}

			var visible bool
			if test.user == "" {
				visible = testPrivateActivityHelperHasVisibleActivitiesFromPublic(t)
			} else {
				visible = testPrivateActivityHelperHasVisibleActivitiesFromSession(t, loginUser(t, test.user))
			}
			if test.visible {
				assert.True(t, visible, "user should have visible activities")
			} else {
				assert.False(t, visible, "user should have no visible activities")
			}
		})
	}
}

func TestPrivateActivityHeatmapVisibility(t *testing.T) {
	for _, test := range []struct {
		name      string
		private   bool
		user      string
		dashboard bool
		visible   bool
	}{
		// check heatmap visibility if the visibility is enabled
		{"NoHeatmapVisibleForPublic", false, "", false, true},
		{"NoHeatmapVisibleForUserItselfAtProfile", false, privateActivityTestUser, false, true},
		{"NoHeatmapVisibleForUserItselfAtDashboard", false, privateActivityTestUser, true, true},
		{"NoHeatmapVisibleForOtherUser", false, privateActivityTestOtherUser, false, true},
		{"NoHeatmapVisibleForAdmin", false, privateActivityTestAdmin, false, true},
		// check heatmap visibility if the visibility is disabled
		{"YesHeatmapInvisibleForPublic", true, "", false, false},
		{"YesHeatmapVisibleForUserItselfAtProfile", true, privateActivityTestUser, false, true},
		{"YesHeatmapVisibleForUserItselfAtDashboard", true, privateActivityTestUser, true, true},
		{"YesHeatmapInvisibleForOtherUser", true, privateActivityTestOtherUser, false, false},
		{"YesHeatmapVisibleForAdmin", true, privateActivityTestAdmin, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()
			testPrivateActivityDoSomethingForActionEntries(t)
			if test.private {
				testPrivateActivityHelperEnablePrivateActivity(t)
			}

			var visible bool
			if test.user == "" {
				visible = testPrivateActivityHelperHasVisibleHeatmapFromPublic(t)
			} else if test.dashboard {
				visible = testPrivateActivityHelperHasVisibleDashboardHeatmapFromSession(t, loginUser(t, test.user))
			} else {
				visible = testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t, loginUser(t, test.user))
			}
			if test.visible {
				assert.True(t, visible, "user should have visible heatmap")
			} else {
				assert.False(t, visible, "user should have no visible heatmap")
			}
		})
	}
}

func TestPrivateActivityHeatmapContent(t *testing.T) {
	for _, test := range []struct {
		name       string
		private    bool
		user       string
		hasContent bool
		message    string
	}{
		// check heatmap api provides content if the visibility is enabled
		{"NoHeatmapHasContentForPublic", false, "", true, "user should have heatmap content"},
		{"NoHeatmapHasContentForUserItself", false, privateActivityTestUser, true, "user should have heatmap content"},
		{"NoHeatmapHasContentForOtherUser", false, privateActivityTestOtherUser, true, "user should have heatmap content"},
		{"NoHeatmapHasContentForAdmin", false, privateActivityTestAdmin, true, "user should have heatmap content"},
		// check heatmap api provides no content if the visibility is disabled
		// this should be equal to the hidden heatmap at the UI
		{"YesHeatmapHasNoContentForPublic", true, "", false, "user should have no heatmap content"},
		{"YesHeatmapHasNoContentForUserItself", true, privateActivityTestUser, true, "user should see their own heatmap content"},
		{"YesHeatmapHasNoContentForOtherUser", true, privateActivityTestOtherUser, false, "other user should not see heatmap content"},
		{"YesHeatmapHasNoContentForAdmin", true, privateActivityTestAdmin, true, "heatmap should show content for admin"},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()
			testPrivateActivityDoSomethingForActionEntries(t)
			if test.private {
				testPrivateActivityHelperEnablePrivateActivity(t)
			}

			var hasContent bool
			if test.user == "" {
				hasContent = testPrivateActivityHelperHasHeatmapContentFromPublic(t)
			} else {
				hasContent = testPrivateActivityHelperHasHeatmapContentFromSession(t, loginUser(t, test.user))
			}
			if test.hasContent {
				assert.True(t, hasContent, test.message)
			} else {
				assert.False(t, hasContent, test.message)
			}
		})
	}
}

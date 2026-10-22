// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/xml"
	"net/http"
	"testing"

	activities_model "gitea.dev/models/activities"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RSS is a struct to unmarshal RSS feeds test only
type RSS struct {
	Channel struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		PubDate     string `xml:"pubDate"`
		Items       []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

func TestFeedUser(t *testing.T) {
	t.Run("User", func(t *testing.T) {
		t.Run("Atom", func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()

			req := NewRequest(t, "GET", "/user2.atom")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<feed xmlns="http://www.w3.org/2005/Atom"`)
		})

		t.Run("RSS", func(t *testing.T) {
			defer tests.PrepareTestEnv(t)()

			req := NewRequest(t, "GET", "/user2.rss")
			resp := MakeRequest(t, req, http.StatusOK)

			data := resp.Body.String()
			assert.Contains(t, data, `<rss version="2.0"`)

			var rss RSS
			err := xml.Unmarshal(resp.Body.Bytes(), &rss)
			assert.NoError(t, err)
			assert.Contains(t, rss.Channel.Link, "/user2")
			assert.NotEmpty(t, rss.Channel.PubDate)
		})
	})

	t.Run("live comment excerpts", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		for _, entry := range []struct {
			opType  activities_model.ActionType
			preview string
			content string
		}{
			{activities_model.ActionCommentIssue, "fresh comment", "1|stale snapshot"},
			{activities_model.ActionPullReviewDismissed, "fresh dismissal", "1|reviewer|stale dismissal"},
		} {
			comment := &issues_model.Comment{
				PosterID: 2,
				IssueID:  1,
				Content:  "```suggestion\nhidden code\n```\n\n" + entry.preview,
			}
			require.NoError(t, db.Insert(t.Context(), comment))
			require.NoError(t, db.Insert(t.Context(), &activities_model.Action{
				UserID:      2,
				ActUserID:   2,
				RepoID:      1,
				CommentID:   comment.ID,
				OpType:      entry.opType,
				Content:     entry.content,
				CreatedUnix: timeutil.TimeStampNow(),
			}))
		}

		for _, requestPath := range []string{"/user2.atom", "/user2.rss", "/user2?tab=activity"} {
			resp := MakeRequest(t, NewRequest(t, "GET", requestPath), http.StatusOK)
			assert.Contains(t, resp.Body.String(), "fresh comment", requestPath)
			assert.Contains(t, resp.Body.String(), "fresh dismissal", requestPath)
			assert.NotContains(t, resp.Body.String(), "stale snapshot", requestPath)
			assert.NotContains(t, resp.Body.String(), "stale dismissal", requestPath)
			assert.NotContains(t, resp.Body.String(), "hidden code", requestPath)
		}
	})

	t.Run("revoked access keeps snapshot previews", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		viewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		require.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), repo, viewer, perm.AccessModeRead))
		comment := &issues_model.Comment{PosterID: 2, IssueID: 4, Content: "original comment"}
		require.NoError(t, db.Insert(t.Context(), comment))
		require.NoError(t, db.Insert(t.Context(), &activities_model.Action{
			UserID:      viewer.ID,
			ActUserID:   2,
			RepoID:      repo.ID,
			CommentID:   comment.ID,
			OpType:      activities_model.ActionCommentIssue,
			IsPrivate:   true,
			Content:     "1|original snapshot",
			CreatedUnix: timeutil.TimeStampNow(),
		}))

		session := loginUser(t, viewer.Name)
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "original comment")
		assert.NotContains(t, resp.Body.String(), "original snapshot")

		require.NoError(t, repo_service.DeleteCollaboration(t.Context(), repo, viewer))
		_, err := db.GetEngine(t.Context()).ID(comment.ID).Cols("content").Update(&issues_model.Comment{Content: "confidential edit"})
		require.NoError(t, err)

		resp = session.MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "original snapshot")
		assert.NotContains(t, resp.Body.String(), "confidential edit")
	})
}

// TestFeedUserPublicOnlyToken ensures a public-only API token cannot surface a user's
// private activity through their profile feed, even when authenticated as the owner.
func TestFeedUserPublicOnlyToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 has activity on the private repo user2/repo2
	const privateMarker = "user2/repo2"

	// a normal read:user token authenticated as the owner sees private activity
	fullToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser)
	reqFull := NewRequest(t, "GET", "/user2.rss")
	reqFull.SetBasicAuth("user2", fullToken)
	respFull := MakeRequest(t, reqFull, http.StatusOK)
	assert.Contains(t, respFull.Body.String(), privateMarker)

	// a public-only token must not surface the private activity
	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)
	reqPublicOnly := NewRequest(t, "GET", "/user2.rss")
	reqPublicOnly.SetBasicAuth("user2", publicOnlyToken)
	respPublicOnly := MakeRequest(t, reqPublicOnly, http.StatusOK)
	assert.NotContains(t, respPublicOnly.Body.String(), privateMarker)
}

// TestProfileActivityPublicOnlyToken ensures the HTML profile activity tab does not
// surface a user's private activity to a public-only API token, mirroring the RSS/Atom
// guard. The /{username} route is AllowBasic, so a public-only PAT used as the Basic
// password must still be downgraded even for the feed owner.
func TestProfileActivityPublicOnlyToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 has activity on the private repo user2/repo2
	const privateMarker = "user2/repo2"

	// a normal read:user token authenticated as the owner sees private activity
	fullToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser)
	reqFull := NewRequest(t, "GET", "/user2?tab=activity")
	reqFull.SetBasicAuth("user2", fullToken)
	respFull := MakeRequest(t, reqFull, http.StatusOK)
	assert.Contains(t, respFull.Body.String(), privateMarker)

	// a public-only token must not surface the private activity
	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)
	reqPublicOnly := NewRequest(t, "GET", "/user2?tab=activity")
	reqPublicOnly.SetBasicAuth("user2", publicOnlyToken)
	respPublicOnly := MakeRequest(t, reqPublicOnly, http.StatusOK)
	assert.NotContains(t, respPublicOnly.Body.String(), privateMarker)
}

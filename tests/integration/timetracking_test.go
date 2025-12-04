// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"path"
	"testing"
	"time"

	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestViewTimetrackingControls(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Exist", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		session := loginUser(t, "user2")
		testViewTimetrackingControls(t, session, 0, "repo1", "1", true, "user2")
	})

	t.Run("Non-exist", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		session := loginUser(t, "user5")
		testViewTimetrackingControls(t, session, 0, "repo1", "1", false, "user2")
	})

	t.Run("Disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		session := loginUser(t, "user2")
		testViewTimetrackingControls(t, session, 129, "repo3", "1", false, "org3")
	})
}

func testViewTimetrackingControls(t *testing.T, session *TestSession, groupID int64, repo, issue string, canTrackTime bool, user string) {
	req := NewRequest(t, "GET", path.Join(user, maybeGroupSegment(groupID), repo, "issues", issue))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	AssertHTMLElement(t, htmlDoc, ".issue-start-time", canTrackTime)
	AssertHTMLElement(t, htmlDoc, ".issue-add-time", canTrackTime)

	issueLink := path.Join(user, maybeGroupSegment(groupID), repo, "issues", issue)
	reqStart := NewRequestWithValues(t, "POST", path.Join(issueLink, "times", "stopwatch", "start"), map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	if canTrackTime {
		session.MakeRequest(t, reqStart, http.StatusOK)

		req = NewRequest(t, "GET", issueLink)
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)

		events := htmlDoc.doc.Find(".event > .comment-text-line")
		assert.Contains(t, events.Last().Text(), "started working")

		AssertHTMLElement(t, htmlDoc, ".issue-stop-time", true)
		AssertHTMLElement(t, htmlDoc, ".issue-cancel-time", true)

		// Sleep for 1 second to not get wrong order for stopping timer
		time.Sleep(time.Second)

		reqStop := NewRequestWithValues(t, "POST", path.Join(issueLink, "times", "stopwatch", "stop"), map[string]string{
			"_csrf": htmlDoc.GetCSRF(),
		})
		session.MakeRequest(t, reqStop, http.StatusOK)

		req = NewRequest(t, "GET", issueLink)
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)

		events = htmlDoc.doc.Find(".event > .comment-text-line")
		assert.Contains(t, events.Last().Text(), "worked for ")
	} else {
		session.MakeRequest(t, reqStart, http.StatusNotFound)
	}
}

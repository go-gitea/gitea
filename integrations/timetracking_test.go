// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestViewTimetrackingControls(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user2")
	testViewTimetrackingControls(t, session, "user2", "repo1", "1", true)
	//user2/repo1
}

func TestNotViewTimetrackingControls(t *testing.T) {
	prepareTestEnv(t)
	session := loginUser(t, "user3")
	testViewTimetrackingControls(t, session, "user2", "repo1", "1", false)
	//user2/repo1
}

func testViewTimetrackingControls(t *testing.T, session *TestSession, user, repo, issue string, canView bool) {
	req := NewRequest(t, "GET", path.Join(user, repo, "issues", issue))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	if start, exists := htmlDoc.doc.Find(".timetrack.start-add-tracking.start-tracking").Attr("onclick"); exists {
		assert.Equal(t, "this.disabled=true;toggleStopwatch()", start)
	} else {
		assert.True(t, (canView && exists) || (!canView || !exists))
	}
	if addTime, exists := htmlDoc.doc.Find(".timetrack.start-add-tracking.add-time").Attr("onclick"); exists {
		assert.Equal(t, "timeAddManual()", addTime)
	} else {
		assert.True(t, (canView && exists) || (!canView || !exists))
	}

	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "issues", issue, "times", "stopwatch", "toggle"), map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	if canView {
		session.MakeRequest(t, req, http.StatusSeeOther)
	} else {
		session.MakeRequest(t, req, http.StatusNotFound)
	}

	req = NewRequest(t, "GET", path.Join(user, repo, "issues", issue))
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	if stop, exists := htmlDoc.doc.Find(".timetrack.stop-cancel-tracking.stop").Attr("onclick"); exists {
		assert.Equal(t, "this.disabled=true;toggleStopwatch()", stop)
	} else {
		assert.True(t, (canView && exists) || (!canView || !exists))
	}
	if cancel, exists := htmlDoc.doc.Find(".timetrack.stop-cancel-tracking.cancel").Attr("onclick"); exists {
		assert.Equal(t, "this.disabled=true;cancelStopwatch()", cancel)
	} else {
		assert.True(t, (canView && exists) || (!canView || !exists))
	}

	req = NewRequestWithValues(t, "POST", path.Join(user, repo, "issues", issue, "times", "stopwatch", "cancel"), map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	if canView {
		session.MakeRequest(t, req, http.StatusSeeOther)
	} else {
		session.MakeRequest(t, req, http.StatusNotFound)
	}

	req = NewRequest(t, "GET", path.Join(user, repo, "issues", issue))
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)

	if start, exists := htmlDoc.doc.Find(".timetrack.start-add-tracking.start-tracking").Attr("onclick"); exists {
		assert.Equal(t, "this.disabled=true;toggleStopwatch()", start)
	} else {
		assert.True(t, (canView && exists) || (!canView || !exists))
	}
	if addTime, exists := htmlDoc.doc.Find(".timetrack.start-add-tracking.add-time").Attr("onclick"); exists {
		assert.Equal(t, "timeAddManual()", addTime)
	} else {
		assert.True(t, (canView && exists) || (!canView || !exists))
	}
}

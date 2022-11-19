// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIIssuesReactions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	_ = issue.LoadRepo(db.DefaultContext)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/reactions?token=%s",
		owner.Name, issue.Repo.Name, issue.Index, token)

	// Try to add not allowed reaction
	req := NewRequestWithJSON(t, "POST", urlStr, &api.EditReactionOption{
		Reaction: "wrong",
	})
	session.MakeRequest(t, req, http.StatusForbidden)

	// Delete not allowed reaction
	req = NewRequestWithJSON(t, "DELETE", urlStr, &api.EditReactionOption{
		Reaction: "zzz",
	})
	session.MakeRequest(t, req, http.StatusOK)

	// Add allowed reaction
	req = NewRequestWithJSON(t, "POST", urlStr, &api.EditReactionOption{
		Reaction: "rocket",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiNewReaction api.Reaction
	DecodeJSON(t, resp, &apiNewReaction)

	// Add existing reaction
	session.MakeRequest(t, req, http.StatusForbidden)

	// Get end result of reaction list of issue #1
	req = NewRequestf(t, "GET", urlStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiReactions []*api.Reaction
	DecodeJSON(t, resp, &apiReactions)
	expectResponse := make(map[int]api.Reaction)
	expectResponse[0] = api.Reaction{
		User:     convert.ToUser(user2, user2),
		Reaction: "eyes",
		Created:  time.Unix(1573248003, 0),
	}
	expectResponse[1] = apiNewReaction
	assert.Len(t, apiReactions, 2)
	for i, r := range apiReactions {
		assert.Equal(t, expectResponse[i].Reaction, r.Reaction)
		assert.Equal(t, expectResponse[i].Created.Unix(), r.Created.Unix())
		assert.Equal(t, expectResponse[i].User.ID, r.User.ID)
	}
}

func TestAPICommentReactions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2})
	_ = comment.LoadIssue(db.DefaultContext)
	issue := comment.Issue
	_ = issue.LoadRepo(db.DefaultContext)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d/reactions?token=%s",
		owner.Name, issue.Repo.Name, comment.ID, token)

	// Try to add not allowed reaction
	req := NewRequestWithJSON(t, "POST", urlStr, &api.EditReactionOption{
		Reaction: "wrong",
	})
	session.MakeRequest(t, req, http.StatusForbidden)

	// Delete none existing reaction
	req = NewRequestWithJSON(t, "DELETE", urlStr, &api.EditReactionOption{
		Reaction: "eyes",
	})
	session.MakeRequest(t, req, http.StatusOK)

	// Add allowed reaction
	req = NewRequestWithJSON(t, "POST", urlStr, &api.EditReactionOption{
		Reaction: "+1",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiNewReaction api.Reaction
	DecodeJSON(t, resp, &apiNewReaction)

	// Add existing reaction
	session.MakeRequest(t, req, http.StatusForbidden)

	// Get end result of reaction list of issue #1
	req = NewRequestf(t, "GET", urlStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiReactions []*api.Reaction
	DecodeJSON(t, resp, &apiReactions)
	expectResponse := make(map[int]api.Reaction)
	expectResponse[0] = api.Reaction{
		User:     convert.ToUser(user2, user2),
		Reaction: "laugh",
		Created:  time.Unix(1573248004, 0),
	}
	expectResponse[1] = api.Reaction{
		User:     convert.ToUser(user1, user1),
		Reaction: "laugh",
		Created:  time.Unix(1573248005, 0),
	}
	expectResponse[2] = apiNewReaction
	assert.Len(t, apiReactions, 3)
	for i, r := range apiReactions {
		assert.Equal(t, expectResponse[i].Reaction, r.Reaction)
		assert.Equal(t, expectResponse[i].Created.Unix(), r.Created.Unix())
		assert.Equal(t, expectResponse[i].User.ID, r.User.ID)
	}
}

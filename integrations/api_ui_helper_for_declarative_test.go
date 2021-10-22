// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/queue"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/forms"
	"github.com/stretchr/testify/assert"
)

type TestContext struct {
	Reponame     string
	Session      *TestSession
	Username     string
	ExpectedCode int
}

func NewTestContext(t *testing.T, username, reponame string) TestContext {
	return TestContext{
		Session:  loginUser(t, username),
		Username: username,
		Reponame: reponame,
	}
}

func (ctx TestContext) GitPath() string {
	return fmt.Sprintf("%s/%s.git", ctx.Username, ctx.Reponame)
}

func (ctx TestContext) CreateAPITestContext(t *testing.T) APITestContext {
	return NewAPITestContext(t, ctx.Username, ctx.Reponame)
}

func doMergePullRequest(ctx TestContext, owner, repo string, index int64) func(*testing.T) {
	return func(t *testing.T) {
		urlStr := fmt.Sprintf("/api/ui/repos/%s/%s/pulls/%d/merge",
			owner, repo, index)
		req := NewRequestWithJSON(t, http.MethodPost, urlStr, &forms.MergePullRequestForm{
			MergeMessageField: "doMergePullRequest Merge",
			Do:                string(models.MergeStyleMerge),
		})

		resp := ctx.Session.MakeRequest(t, req, NoExpectedStatus)

		if resp.Code == http.StatusMethodNotAllowed {
			err := api.APIError{}
			DecodeJSON(t, resp, &err)
			assert.EqualValues(t, "Please try again later", err.Message)
			queue.GetManager().FlushAll(context.Background(), 5*time.Second)
			req = NewRequestWithJSON(t, http.MethodPost, urlStr, &forms.MergePullRequestForm{
				MergeMessageField: "doMergePullRequest Merge",
				Do:                string(models.MergeStyleMerge),
			})
			resp = ctx.Session.MakeRequest(t, req, NoExpectedStatus)
		}

		expected := ctx.ExpectedCode
		if expected == 0 {
			expected = 200
		}

		if !assert.EqualValues(t, expected, resp.Code,
			"Request: %s %s", req.Method, req.URL.String()) {
			logUnexpectedResponse(t, resp)
		}
	}
}

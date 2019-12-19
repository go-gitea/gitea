// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackIssuesPayload(t *testing.T) {
	p := &api.IssuePayload{
		Index:  1,
		Action: api.HookIssueClosed,
		Sender: &api.User{
			UserName: "user1",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Issue: &api.Issue{
			ID:    2,
			URL:   "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			Title: "crash",
		},
	}

	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	pl, err := getSlackIssuesPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Issue closed: <http://localhost:3000/api/v1/repos/test/repo/issues/2|#1 crash> by <https://try.gitea.io/user1|user1>", pl.Text)
}

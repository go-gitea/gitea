// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackIssuesPayload(t *testing.T) {
	p := issueTestPayLoad()

	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	pl, err := getSlackIssuesPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Issue closed: <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.Text)
}

func TestSlackIssueCommentPayload(t *testing.T) {
	p := issueCommentTestPayLoad()

	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	pl, err := getSlackIssueCommentPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] New comment on issue <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.Text)
}

func TestSlackPullRequestCommentPayload(t *testing.T) {
	p := pullRequestCommentTestPayLoad()

	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	pl, err := getSlackIssueCommentPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] New comment on pull request <http://localhost:3000/test/repo/pulls/2|#2 Fix bug> by <https://try.gitea.io/user1|user1>", pl.Text)
}

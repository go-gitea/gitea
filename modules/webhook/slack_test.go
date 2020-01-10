// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackIssuesPayloadOpened(t *testing.T) {
	p := issueTestPayload()
	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	p.Action = api.HookIssueOpened
	pl, err := getSlackIssuesPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)
	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Issue opened: <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.Text)

	p.Action = api.HookIssueClosed
	pl, err = getSlackIssuesPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)
	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Issue closed: <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.Text)
}

func TestSlackIssueCommentPayload(t *testing.T) {
	p := issueCommentTestPayload()

	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	pl, err := getSlackIssueCommentPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] New comment on issue <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.Text)
}

func TestSlackPullRequestCommentPayload(t *testing.T) {
	p := pullRequestCommentTestPayload()

	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	pl, err := getSlackIssueCommentPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] New comment on pull request <http://localhost:3000/test/repo/pulls/2|#2 Fix bug> by <https://try.gitea.io/user1|user1>", pl.Text)
}

func TestSlackReleasePayload(t *testing.T) {
	p := pullReleaseTestPayload()

	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	pl, err := getSlackReleasePayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Release created: <http://localhost:3000/test/repo/src/v1.0|v1.0> by <https://try.gitea.io/user1|user1>", pl.Text)
}

func TestSlackPullRequestPayload(t *testing.T) {
	p := pullRequestTestPayload()

	sl := &SlackMeta{
		Username: p.Sender.UserName,
	}

	pl, err := getSlackPullRequestPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Pull request opened: <http://localhost:3000/test/repo/pulls/12|#2 Fix bug> by <https://try.gitea.io/user1|user1>", pl.Text)
}

// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatrixIssuesPayloadOpened(t *testing.T) {
	p := issueTestPayload()
	sl := &MatrixMeta{}

	p.Action = api.HookIssueOpened
	pl, err := getMatrixIssuesPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)
	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue opened: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Issue opened: <a href=\"http://localhost:3000/test/repo/issues/2\">#2 crash</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)

	p.Action = api.HookIssueClosed
	pl, err = getMatrixIssuesPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)
	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue closed: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Issue closed: <a href=\"http://localhost:3000/test/repo/issues/2\">#2 crash</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixIssueCommentPayload(t *testing.T) {
	p := issueCommentTestPayload()

	sl := &MatrixMeta{}

	pl, err := getMatrixIssueCommentPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on issue [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] New comment on issue <a href=\"http://localhost:3000/test/repo/issues/2\">#2 crash</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixPullRequestCommentPayload(t *testing.T) {
	p := pullRequestCommentTestPayload()

	sl := &MatrixMeta{}

	pl, err := getMatrixIssueCommentPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on pull request [#2 Fix bug](http://localhost:3000/test/repo/pulls/2) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] New comment on pull request <a href=\"http://localhost:3000/test/repo/pulls/2\">#2 Fix bug</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixReleasePayload(t *testing.T) {
	p := pullReleaseTestPayload()

	sl := &MatrixMeta{}

	pl, err := getMatrixReleasePayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Release created: [v1.0](http://localhost:3000/test/repo/src/v1.0) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Release created: <a href=\"http://localhost:3000/test/repo/src/v1.0\">v1.0</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixPullRequestPayload(t *testing.T) {
	p := pullRequestTestPayload()

	sl := &MatrixMeta{}

	pl, err := getMatrixPullRequestPayload(p, sl)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Pull request opened: [#2 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Pull request opened: <a href=\"http://localhost:3000/test/repo/pulls/12\">#2 Fix bug</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

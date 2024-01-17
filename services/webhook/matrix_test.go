// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatrixPayload(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		d := new(MatrixPayload)
		pl, err := d.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo):[test](http://localhost:3000/test/repo/src/branch/test)] branch created by user1", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>:<a href="http://localhost:3000/test/repo/src/branch/test">test</a>] branch created by user1`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		d := new(MatrixPayload)
		pl, err := d.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo):test] branch deleted by user1", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>:test] branch deleted by user1`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		d := new(MatrixPayload)
		pl, err := d.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[test/repo2](http://localhost:3000/test/repo2) is forked to [test/repo](http://localhost:3000/test/repo)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `<a href="http://localhost:3000/test/repo2">test/repo2</a> is forked to <a href="http://localhost:3000/test/repo">test/repo</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		d := new(MatrixPayload)
		pl, err := d.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] user1 pushed 2 commits to [test](http://localhost:3000/test/repo/src/branch/test):\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778): commit message - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778): commit message - user1", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] user1 pushed 2 commits to <a href="http://localhost:3000/test/repo/src/branch/test">test</a>:<br><a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778">2020558</a>: commit message - user1<br><a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778">2020558</a>: commit message - user1`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		d := new(MatrixPayload)
		p.Action = api.HookIssueOpened
		pl, err := d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue opened: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Issue opened: <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)

		p.Action = api.HookIssueClosed
		pl, err = d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue closed: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Issue closed: <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		d := new(MatrixPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on issue [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] New comment on issue <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		d := new(MatrixPayload)
		pl, err := d.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Pull request opened: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Pull request opened: <a href="http://localhost:3000/test/repo/pulls/12">#12 Fix bug</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		d := new(MatrixPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on pull request [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] New comment on pull request <a href="http://localhost:3000/test/repo/pulls/12">#12 Fix bug</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		d := new(MatrixPayload)
		pl, err := d.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Pull request review approved: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Pull request review approved: <a href="http://localhost:3000/test/repo/pulls/12">#12 Fix bug</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		d := new(MatrixPayload)
		pl, err := d.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, `[[test/repo](http://localhost:3000/test/repo)] Repository created by [user1](https://try.gitea.io/user1)`, pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Repository created by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		d := new(MatrixPayload)
		pl, err := d.Package(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, `[[GiteaContainer](http://localhost:3000/user1/-/packages/container/GiteaContainer/latest)] Package published by [user1](https://try.gitea.io/user1)`, pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/user1/-/packages/container/GiteaContainer/latest">GiteaContainer</a>] Package published by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		d := new(MatrixPayload)
		p.Action = api.HookWikiCreated
		pl, err := d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New wiki page '[index](http://localhost:3000/test/repo/wiki/index)' (Wiki change comment) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] New wiki page '<a href="http://localhost:3000/test/repo/wiki/index">index</a>' (Wiki change comment) by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)

		p.Action = api.HookWikiEdited
		pl, err = d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Wiki page '[index](http://localhost:3000/test/repo/wiki/index)' edited (Wiki change comment) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Wiki page '<a href="http://localhost:3000/test/repo/wiki/index">index</a>' edited (Wiki change comment) by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)

		p.Action = api.HookWikiDeleted
		pl, err = d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Wiki page '[index](http://localhost:3000/test/repo/wiki/index)' deleted by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Wiki page '<a href="http://localhost:3000/test/repo/wiki/index">index</a>' deleted by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		d := new(MatrixPayload)
		pl, err := d.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayload{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Release created: [v1.0](http://localhost:3000/test/repo/releases/tag/v1.0) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayload).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Release created: <a href="http://localhost:3000/test/repo/releases/tag/v1.0">v1.0</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayload).FormattedBody)
	})
}

func TestMatrixJSONPayload(t *testing.T) {
	p := pushTestPayload()

	pl, err := new(MatrixPayload).Push(p)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.IsType(t, &MatrixPayload{}, pl)

	json, err := pl.JSONPayload()
	require.NoError(t, err)
	assert.NotEmpty(t, json)
}

func Test_getTxnID(t *testing.T) {
	type args struct {
		payload []byte
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "dummy payload",
			args:    args{payload: []byte("Hello World")},
			want:    "0a4d55a8d778e5022fab701977c5d840bbc486d0",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getMatrixTxnID(tt.args.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("getMatrixTxnID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

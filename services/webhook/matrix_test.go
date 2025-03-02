// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatrixPayload(t *testing.T) {
	mc := matrixConvertor{
		MsgType: "m.text",
	}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := mc.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo):[test](http://localhost:3000/test/repo/src/branch/test)] branch created by user1", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>:<a href="http://localhost:3000/test/repo/src/branch/test">test</a>] branch created by user1`, pl.FormattedBody)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := mc.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo):test] branch deleted by user1", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>:test] branch deleted by user1`, pl.FormattedBody)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := mc.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[test/repo2](http://localhost:3000/test/repo2) is forked to [test/repo](http://localhost:3000/test/repo)", pl.Body)
		assert.Equal(t, `<a href="http://localhost:3000/test/repo2">test/repo2</a> is forked to <a href="http://localhost:3000/test/repo">test/repo</a>`, pl.FormattedBody)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := mc.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] user1 pushed 2 commits to [test](http://localhost:3000/test/repo/src/branch/test):\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778): commit message - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778): commit message - user1", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] user1 pushed 2 commits to <a href="http://localhost:3000/test/repo/src/branch/test">test</a>:<br><a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778">2020558</a>: commit message - user1<br><a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778">2020558</a>: commit message - user1`, pl.FormattedBody)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := mc.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue opened: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Issue opened: <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)

		p.Action = api.HookIssueClosed
		pl, err = mc.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue closed: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Issue closed: <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := mc.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on issue [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] New comment on issue <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := mc.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Pull request opened: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Pull request opened: <a href="http://localhost:3000/test/repo/pulls/12">#12 Fix bug</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := mc.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on pull request [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] New comment on pull request <a href="http://localhost:3000/test/repo/pulls/12">#12 Fix bug</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := mc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Pull request review approved: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Pull request review approved: <a href="http://localhost:3000/test/repo/pulls/12">#12 Fix bug</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := mc.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, `[[test/repo](http://localhost:3000/test/repo)] Repository created by [user1](https://try.gitea.io/user1)`, pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Repository created by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := mc.Package(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, `[[GiteaContainer](http://localhost:3000/user1/-/packages/container/GiteaContainer/latest)] Package published by [user1](https://try.gitea.io/user1)`, pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/user1/-/packages/container/GiteaContainer/latest">GiteaContainer</a>] Package published by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := mc.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New wiki page '[index](http://localhost:3000/test/repo/wiki/index)' (Wiki change comment) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] New wiki page '<a href="http://localhost:3000/test/repo/wiki/index">index</a>' (Wiki change comment) by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)

		p.Action = api.HookWikiEdited
		pl, err = mc.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Wiki page '[index](http://localhost:3000/test/repo/wiki/index)' edited (Wiki change comment) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Wiki page '<a href="http://localhost:3000/test/repo/wiki/index">index</a>' edited (Wiki change comment) by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)

		p.Action = api.HookWikiDeleted
		pl, err = mc.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Wiki page '[index](http://localhost:3000/test/repo/wiki/index)' deleted by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Wiki page '<a href="http://localhost:3000/test/repo/wiki/index">index</a>' deleted by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := mc.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Release created: [v1.0](http://localhost:3000/test/repo/releases/tag/v1.0) by [user1](https://try.gitea.io/user1)", pl.Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Release created: <a href="http://localhost:3000/test/repo/releases/tag/v1.0">v1.0</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.FormattedBody)
	})
}

func TestMatrixJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:   3,
		IsActive: true,
		Type:     webhook_module.MATRIX,
		URL:      "https://matrix.example.com/_matrix/client/r0/rooms/ROOM_ID/send/m.room.message",
		Meta:     `{"message_type":0}`, // text
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newMatrixRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "PUT", req.Method)
	assert.Equal(t, "/_matrix/client/r0/rooms/ROOM_ID/send/m.room.message/6db5dc1e282529a8c162c7fe93dd2667494eeb51", req.URL.Path)
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body MatrixPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	assert.NoError(t, err)
	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] user1 pushed 2 commits to [test](http://localhost:3000/test/repo/src/branch/test):\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778): commit message - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778): commit message - user1", body.Body)
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

// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatrixIssuesPayloadOpened(t *testing.T) {
	p := issueTestPayload()
	sl := &MatrixMeta{}

	p.Action = api.HookIssueOpened
	pl, err := getMatrixIssuesPayload(p, sl)
	require.NoError(t, err)
	require.NotNil(t, pl)
	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue opened: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Issue opened: <a href=\"http://localhost:3000/test/repo/issues/2\">#2 crash</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)

	p.Action = api.HookIssueClosed
	pl, err = getMatrixIssuesPayload(p, sl)
	require.NoError(t, err)
	require.NotNil(t, pl)
	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue closed: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Issue closed: <a href=\"http://localhost:3000/test/repo/issues/2\">#2 crash</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixIssueCommentPayload(t *testing.T) {
	p := issueCommentTestPayload()

	sl := &MatrixMeta{}

	pl, err := getMatrixIssueCommentPayload(p, sl)
	require.NoError(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on issue [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] New comment on issue <a href=\"http://localhost:3000/test/repo/issues/2\">#2 crash</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixPullRequestCommentPayload(t *testing.T) {
	p := pullRequestCommentTestPayload()

	sl := &MatrixMeta{}

	pl, err := getMatrixIssueCommentPayload(p, sl)
	require.NoError(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on pull request [#2 Fix bug](http://localhost:3000/test/repo/pulls/2) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] New comment on pull request <a href=\"http://localhost:3000/test/repo/pulls/2\">#2 Fix bug</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixReleasePayload(t *testing.T) {
	p := pullReleaseTestPayload()

	sl := &MatrixMeta{}

	pl, err := getMatrixReleasePayload(p, sl)
	require.NoError(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Release created: [v1.0](http://localhost:3000/test/repo/src/v1.0) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Release created: <a href=\"http://localhost:3000/test/repo/src/v1.0\">v1.0</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixPullRequestPayload(t *testing.T) {
	p := pullRequestTestPayload()

	sl := &MatrixMeta{}

	pl, err := getMatrixPullRequestPayload(p, sl)
	require.NoError(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Pull request opened: [#2 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.Body)
	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Pull request opened: <a href=\"http://localhost:3000/test/repo/pulls/12\">#2 Fix bug</a> by <a href=\"https://try.gitea.io/user1\">user1</a>", pl.FormattedBody)
}

func TestMatrixHookRequest(t *testing.T) {
	h := &models.HookTask{
		PayloadContent: `{
  "body": "[[user1/test](http://localhost:3000/user1/test)] user1 pushed 1 commit to [master](http://localhost:3000/user1/test/src/branch/master):\n[5175ef2](http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee): Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n - user1",
  "msgtype": "m.notice",
  "format": "org.matrix.custom.html",
  "formatted_body": "[\u003ca href=\"http://localhost:3000/user1/test\"\u003euser1/test\u003c/a\u003e] user1 pushed 1 commit to \u003ca href=\"http://localhost:3000/user1/test/src/branch/master\"\u003emaster\u003c/a\u003e:\u003cbr\u003e\u003ca href=\"http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee\"\u003e5175ef2\u003c/a\u003e: Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n - user1",
  "io.gitea.commits": [
    {
      "id": "5175ef26201c58b035a3404b3fe02b4e8d436eee",
      "message": "Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n",
      "url": "http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee",
      "author": {
        "name": "user1",
        "email": "user@mail.com",
        "username": ""
      },
      "committer": {
        "name": "user1",
        "email": "user@mail.com",
        "username": ""
      },
      "verification": null,
      "timestamp": "0001-01-01T00:00:00Z",
      "added": null,
      "removed": null,
      "modified": null
    }
  ],
  "access_token": "dummy_access_token"
}`,
	}

	wantPayloadContent := `{
  "body": "[[user1/test](http://localhost:3000/user1/test)] user1 pushed 1 commit to [master](http://localhost:3000/user1/test/src/branch/master):\n[5175ef2](http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee): Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n - user1",
  "msgtype": "m.notice",
  "format": "org.matrix.custom.html",
  "formatted_body": "[\u003ca href=\"http://localhost:3000/user1/test\"\u003euser1/test\u003c/a\u003e] user1 pushed 1 commit to \u003ca href=\"http://localhost:3000/user1/test/src/branch/master\"\u003emaster\u003c/a\u003e:\u003cbr\u003e\u003ca href=\"http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee\"\u003e5175ef2\u003c/a\u003e: Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n - user1",
  "io.gitea.commits": [
    {
      "id": "5175ef26201c58b035a3404b3fe02b4e8d436eee",
      "message": "Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n",
      "url": "http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee",
      "author": {
        "name": "user1",
        "email": "user@mail.com",
        "username": ""
      },
      "committer": {
        "name": "user1",
        "email": "user@mail.com",
        "username": ""
      },
      "verification": null,
      "timestamp": "0001-01-01T00:00:00Z",
      "added": null,
      "removed": null,
      "modified": null
    }
  ]
}`

	req, err := getMatrixHookRequest(h)
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, "Bearer dummy_access_token", req.Header.Get("Authorization"))
	assert.Equal(t, wantPayloadContent, h.PayloadContent)
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

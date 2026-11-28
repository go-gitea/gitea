// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/storage"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffContentHistory(t *testing.T) {
	out := diffContentHistory("<\r\n&\r\n>", "<\nXXX\n>")
	assert.Equal(t, `<pre class="chroma">&lt;
<span class="gd">&amp;</span><span class="gi">XXX</span>
&gt;</pre>`, string(out))
}

func TestUnpublishedReviewCommentNotLeaked(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// comment 4 of pull request user2/repo1#2 belongs to review 4, which user1 has not submitted yet
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 4})
	require.NoError(t, issues_model.SaveIssueContentHistory(t.Context(), comment.PosterID, comment.IssueID, comment.ID, 946684812, "draft note", true))
	require.NoError(t, issues_model.SaveIssueContentHistory(t.Context(), comment.PosterID, comment.IssueID, comment.ID, 946684813, "draft note with a secret", false))
	items, err := issues_model.FetchIssueContentHistoryList(t.Context(), comment.IssueID, comment.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)

	attach := &repo_model.Attachment{UUID: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a44", RepoID: 1, IssueID: comment.IssueID, CommentID: comment.ID, UploaderID: comment.PosterID, Name: "draft.txt", Size: 6}
	require.NoError(t, db.Insert(t.Context(), attach))
	_, err = storage.Attachments.Save(attach.RelativePath(), strings.NewReader("secret"), attach.Size)
	require.NoError(t, err)

	mockContext := func(doerID int64) (*context.Context, *httptest.ResponseRecorder) {
		ctx, resp := contexttest.MockContext(t, "user2/repo1/pulls/2/content-history/detail")
		if doerID != 0 {
			contexttest.LoadUser(t, ctx, doerID)
		}
		contexttest.LoadRepo(t, ctx, 1)
		ctx.SetPathParam("index", "2")
		ctx.SetPathParam("id", strconv.FormatInt(comment.ID, 10))
		ctx.Req.Form.Set("comment_id", strconv.FormatInt(comment.ID, 10))
		ctx.Req.Form.Set("history_id", strconv.FormatInt(items[0].HistoryID, 10))
		return ctx, resp
	}

	// the reviewer still sees their own unsubmitted comment
	for _, c := range []struct {
		handler func(*context.Context)
		want    string
	}{
		{GetContentHistoryOverview, `"editedHistoryCountMap":{"` + strconv.FormatInt(comment.ID, 10) + `":2}`},
		{GetContentHistoryList, `"value":` + strconv.FormatInt(items[0].HistoryID, 10)},
		{GetContentHistoryDetail, "with a secret"},
		{GetCommentAttachments, attach.Name},
	} {
		ctx, resp := mockContext(comment.PosterID)
		c.handler(ctx)
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Contains(t, resp.Body.String(), c.want)
	}
	ctx, resp := mockContext(comment.PosterID)
	ServeAttachment(ctx, attach.UUID)
	assert.Equal(t, "secret", resp.Body.String())

	// the repository owner and an unrelated reader, signed in or not, see nothing of it
	for _, doerID := range []int64{0, 2, 4} {
		ctx, resp := mockContext(doerID)
		GetContentHistoryOverview(ctx)
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.NotContains(t, resp.Body.String(), `"editedHistoryCountMap":{"`+strconv.FormatInt(comment.ID, 10))

		for _, handler := range []func(*context.Context){GetContentHistoryList, GetContentHistoryDetail, GetCommentAttachments} {
			ctx, resp := mockContext(doerID)
			handler(ctx)
			assert.Equal(t, http.StatusNotFound, resp.Code, "doer %d", doerID)
			assert.NotContains(t, resp.Body.String(), "secret")
		}

		ctx, resp = mockContext(doerID)
		ServeAttachment(ctx, attach.UUID)
		assert.Equal(t, http.StatusNotFound, resp.Code, "doer %d", doerID)
		assert.NotContains(t, resp.Body.String(), "secret")
	}
}

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"html/template"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/translation"

	"github.com/stretchr/testify/assert"
)

func TestRenderTimelineEventComment(t *testing.T) {
	ctx := reqctx.NewRequestContextForTest(t.Context())
	ctx.SetContextValue(translation.ContextKey, &translation.MockLocale{})
	ut := &RenderUtils{ctx: ctx}
	var createdStr template.HTML = "(created-at)"

	c := &issues_model.Comment{Type: issues_model.CommentTypeChangeTitle, OldTitle: "WIP: title", NewTitle: "title"}
	assert.Equal(t, "repo.pulls.marked_as_ready_for_review_at:(created-at)", string(ut.RenderTimelineEventComment(c, createdStr)))

	c = &issues_model.Comment{Type: issues_model.CommentTypeChangeTitle, OldTitle: "title", NewTitle: "WIP: title"}
	assert.Equal(t, "repo.pulls.marked_as_work_in_progress_at:(created-at)", string(ut.RenderTimelineEventComment(c, createdStr)))

	c = &issues_model.Comment{Type: issues_model.CommentTypeChangeTitle, OldTitle: "title", NewTitle: "WIP: new title"}
	assert.Equal(t, "repo.issues.change_title_at:title,WIP: new title,(created-at)", string(ut.RenderTimelineEventComment(c, createdStr)))
}

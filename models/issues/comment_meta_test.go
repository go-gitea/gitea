// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"testing"

	project_model "gitea.dev/models/project"

	"github.com/stretchr/testify/assert"
)

func TestBuildCreateCommentMetaData(t *testing.T) {
	ctx := t.Context()
	columnOpts := &CreateCommentOptions{ProjectColumnID: 5, ProjectColumnTitle: "Done", ProjectTitle: "Board"}

	assert.Nil(t, buildCreateCommentMetaData(ctx, &CreateCommentOptions{}))
	assert.Equal(t, &CommentMetaData{ProjectColumnID: 5, ProjectColumnTitle: "Done", ProjectTitle: "Board"}, buildCreateCommentMetaData(ctx, columnOpts))
	assert.False(t, IsProjectWorkflowContext(ctx))

	ctx = WithProjectWorkflow(ctx, project_model.WorkflowEventItemClosed)
	assert.True(t, IsProjectWorkflowContext(ctx))
	assert.Equal(t, &CommentMetaData{
		ProjectColumnID:      5,
		ProjectColumnTitle:   "Done",
		ProjectTitle:         "Board",
		ProjectWorkflowEvent: project_model.WorkflowEventItemClosed,
		SpecialDoerName:      SpecialDoerNameProjectWorkflow,
	}, buildCreateCommentMetaData(ctx, columnOpts))
}

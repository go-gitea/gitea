// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"testing"

	project_model "gitea.dev/models/project"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestBuildCreateCommentMetaData(t *testing.T) {
	// No special data: nil must be returned (zero-value metadata is avoided on purpose).
	meta := buildCreateCommentMetaData(&CreateCommentOptions{
		Doer: &user_model.User{ID: 1},
	})
	assert.Nil(t, meta)

	// ProjectColumnTitle triggers the column metadata branch.
	meta = buildCreateCommentMetaData(&CreateCommentOptions{
		Doer:               &user_model.User{ID: 1},
		ProjectColumnID:    5,
		ProjectColumnTitle: "In Progress",
		ProjectTitle:       "My Project",
	})
	assert.NotNil(t, meta)
	assert.Equal(t, int64(5), meta.ProjectColumnID)
	assert.Equal(t, "In Progress", meta.ProjectColumnTitle)
	assert.Equal(t, "My Project", meta.ProjectTitle)

	// SpecialDoerName (e.g. CODEOWNERS) stores only the name.
	meta = buildCreateCommentMetaData(&CreateCommentOptions{
		Doer:            &user_model.User{ID: 1},
		SpecialDoerName: SpecialDoerNameCodeOwners,
	})
	assert.NotNil(t, meta)
	assert.Equal(t, SpecialDoerNameCodeOwners, meta.SpecialDoerName)
	assert.Zero(t, meta.ProjectWorkflowID)

	// ExtDoerData must be stored as *projectWorkflowDoer (pointer); a value type
	// would not match the type assertion and all workflow fields would silently be zero.
	const (
		wfID      = int64(42)
		wfEvent   = project_model.WorkflowEventItemOpened
		projTitle = "Kanban"
	)
	workflowDoer := NewProjectWorkflowDoer(projTitle, wfID, wfEvent)
	meta = buildCreateCommentMetaData(&CreateCommentOptions{Doer: workflowDoer})
	assert.NotNil(t, meta)
	assert.Equal(t, SpecialDoerNameProjectWorkflow, meta.SpecialDoerName)
	assert.Equal(t, wfID, meta.ProjectWorkflowID)
	assert.Equal(t, wfEvent, meta.ProjectWorkflowEvent)
	assert.Equal(t, projTitle, meta.ProjectTitle)

	// Passing a value-type projectWorkflowDoer (not pointer) must NOT match
	// the *projectWorkflowDoer assertion, so metadata must remain nil.
	nilMetaDoer := &user_model.User{
		ID: 1,
		ExtDoerData: projectWorkflowDoer{ // value, not *projectWorkflowDoer
			projectTitle:         "WrongTitle",
			projectWorkflowID:    99,
			projectWorkflowEvent: project_model.WorkflowEventItemClosed,
		},
	}
	meta = buildCreateCommentMetaData(&CreateCommentOptions{Doer: nilMetaDoer})
	assert.Nil(t, meta, "value-type projectWorkflowDoer must not match *projectWorkflowDoer type assertion")
	assert.True(t, IsProjectWorkflowDoer(workflowDoer))
	assert.False(t, IsProjectWorkflowDoer(nilMetaDoer))
	assert.False(t, IsProjectWorkflowDoer(&user_model.User{ID: 1}))
}

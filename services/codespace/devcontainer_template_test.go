// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevContainerTemplateSettingsScope(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	global := insertServiceDevContainerTemplate(t, 0, "Global", `{"image":"debian:12"}`)
	personal := insertServiceDevContainerTemplate(t, 2, "Personal", `{"image":"node:24"}`)
	foreign := insertServiceDevContainerTemplate(t, 4, "Foreign", `{"image":"golang:1.25"}`)

	visible, err := listVisibleDevContainerTemplates(t.Context(), 2)
	require.NoError(t, err)
	require.Len(t, visible, 2)
	assert.Equal(t, []int64{global.ID, personal.ID}, []int64{visible[0].ID, visible[1].ID})

	require.ErrorIs(t, UpsertDevContainerTemplate(t.Context(), DevContainerTemplateUpsertOptions{
		UserID:  2,
		ID:      foreign.ID,
		Name:    "Changed",
		Content: `{"image":"debian:13"}`,
	}), ErrDevContainerTemplateNotFound)

	require.ErrorIs(t, DeleteDevContainerTemplate(t.Context(), DevContainerTemplateDeleteOptions{
		UserID: 2,
		ID:     personal.ID,
	}), ErrDevContainerTemplateConfirmRequired)
	require.NoError(t, DeleteDevContainerTemplate(t.Context(), DevContainerTemplateDeleteOptions{
		UserID:  2,
		ID:      personal.ID,
		Confirm: true,
	}))
	unittest.AssertNotExistsBean(t, &codespace_model.DevContainerTemplate{ID: personal.ID})
}

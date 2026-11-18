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

func TestDevContainerTemplateSettings(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	global := insertServiceDevContainerTemplate(t, 0, "Global", `{"image":"debian:12"}`)
	content := `{
		// Runtime resolves the build and mounts.
		"build": {"dockerfile": "Dockerfile"},
		"mounts": ["type=volume,source=data,target=/data"],
		"secrets": {"API_TOKEN": {"description": "Service token"}},
		"customizations": {
			"gitea": {"environmentTag": "standard", "repositories": {"user2/repo2": {"permissions": {"code": "read"}}}},
			"editor": {"settings": {"theme": "dark"}}
		}
	}`
	require.NoError(t, UpsertDevContainerTemplate(t.Context(), DevContainerTemplateUpsertOptions{
		UserID: 2, Name: "Personal", Content: content,
	}))
	personal := unittest.AssertExistsAndLoadBean(t, &codespace_model.DevContainerTemplate{UserID: 2, Name: "Personal"})
	assert.Equal(t, content, personal.Content)
	plan, err := loadTemplateDevContainer(personal)
	require.NoError(t, err)
	assert.Equal(t, content, plan.Content)
	assert.Equal(t, "standard", plan.EnvironmentTag)
	assert.Equal(t, map[string]map[string]string{"user2/repo2": {"code": "read"}}, plan.PermissionRepositories)
	assert.Equal(t, []CreateRecommendedSecret{{Name: "API_TOKEN", Description: "Service token"}}, plan.RecommendedSecrets)

	foreign := insertServiceDevContainerTemplate(t, 4, "Foreign", `{"image":"golang:1.25"}`)

	visible, err := listVisibleDevContainerTemplates(t.Context(), 2)
	require.NoError(t, err)
	require.Len(t, visible, 2)
	assert.Equal(t, []int64{global.ID, personal.ID}, []int64{visible[0].ID, visible[1].ID})

	for _, invalid := range []string{
		`null`,
		`[]`,
		`{"secrets": []}`,
		`{"customizations": {"gitea": {"environmentTag": 123}}}`,
		`{"customizations": {"gitea": {"environmentTag": "invalid tag"}}}`,
		`{"customizations": {"gitea": {"repositories": {"user2/repo2": {"permissions": {"code": "admin"}}}}}}`,
	} {
		require.ErrorIs(t, UpsertDevContainerTemplate(t.Context(), DevContainerTemplateUpsertOptions{
			UserID: 2, ID: personal.ID, Name: "Personal", Content: invalid,
		}), ErrCreateConfigurationInvalid, "content: %s", invalid)
	}

	require.ErrorIs(t, UpsertDevContainerTemplate(t.Context(), DevContainerTemplateUpsertOptions{
		UserID:  2,
		ID:      foreign.ID,
		Name:    "Changed",
		Content: `{"image":"debian:13"}`,
	}), ErrDevContainerTemplateNotFound)

	require.NoError(t, DeleteDevContainerTemplate(t.Context(), DevContainerTemplateDeleteOptions{
		UserID: 2,
		ID:     personal.ID,
	}))
	unittest.AssertNotExistsBean(t, &codespace_model.DevContainerTemplate{ID: personal.ID})
}

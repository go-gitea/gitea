// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func TestCreateEnvironmentOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    CreateEnvironmentOptions
		wantErr bool
	}{
		{
			name: "valid options",
			opts: CreateEnvironmentOptions{
				RepoID:      1,
				Name:        "production",
				Description: "Production environment",
				CreatedByID: 1,
			},
			wantErr: false,
		},
		{
			name: "empty name",
			opts: CreateEnvironmentOptions{
				RepoID:      1,
				Name:        "",
				CreatedByID: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid repo ID",
			opts: CreateEnvironmentOptions{
				RepoID:      0,
				Name:        "test",
				CreatedByID: 1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnvironmentCRUD(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const repoID = 1
	const userID = 1

	// Test Create
	opts := CreateEnvironmentOptions{
		RepoID:      repoID,
		Name:        "test-env",
		Description: "Test environment",
		ExternalURL: "https://test.example.com",
		CreatedByID: userID,
	}

	env, err := CreateEnvironment(db.DefaultContext, opts)
	require.NoError(t, err)
	assert.Equal(t, opts.Name, env.Name)
	assert.Equal(t, opts.Description, env.Description)
	assert.Equal(t, opts.ExternalURL, env.ExternalURL)
	assert.Equal(t, opts.RepoID, env.RepoID)
	assert.Equal(t, opts.CreatedByID, env.CreatedByID)

	// Test Get
	retrievedEnv, err := GetEnvironmentByRepoIDAndName(db.DefaultContext, repoID, "test-env")
	require.NoError(t, err)
	assert.Equal(t, env.ID, retrievedEnv.ID)
	assert.Equal(t, env.Name, retrievedEnv.Name)

	// Test Find
	envs, err := FindEnvironments(db.DefaultContext, FindEnvironmentsOptions{
		RepoID: repoID,
	})
	require.NoError(t, err)
	assert.Len(t, envs, 1)
	assert.Equal(t, env.ID, envs[0].ID)

	// Test Update
	updateOpts := UpdateEnvironmentOptions{
		Description: func() *string { s := "Updated description"; return &s }(),
	}
	err = UpdateEnvironment(db.DefaultContext, env, updateOpts)
	require.NoError(t, err)

	// Verify update
	updatedEnv, err := GetEnvironmentByRepoIDAndName(db.DefaultContext, repoID, "test-env")
	require.NoError(t, err)
	assert.Equal(t, "Updated description", updatedEnv.Description)

	// Test Count
	count, err := CountEnvironments(db.DefaultContext, repoID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Test Check Exists
	exists, err := CheckEnvironmentExists(db.DefaultContext, repoID, "test-env")
	require.NoError(t, err)
	assert.True(t, exists)

	// Test Delete
	err = DeleteEnvironment(db.DefaultContext, repoID, "test-env")
	require.NoError(t, err)

	// Verify deletion
	_, err = GetEnvironmentByRepoIDAndName(db.DefaultContext, repoID, "test-env")
	assert.Error(t, err)

	exists, err = CheckEnvironmentExists(db.DefaultContext, repoID, "test-env")
	require.NoError(t, err)
	assert.False(t, exists)

	count, err = CountEnvironments(db.DefaultContext, repoID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestCreateOrGetEnvironmentByName(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const repoID = 1
	const userID = 1

	// Test creating new environment
	env, err := CreateOrGetEnvironmentByName(db.DefaultContext, repoID, "new-env", userID, "https://new.example.com")
	require.NoError(t, err)
	assert.Equal(t, "new-env", env.Name)
	assert.Equal(t, int64(repoID), env.RepoID)
	assert.Equal(t, int64(userID), env.CreatedByID)
	assert.Equal(t, "Auto-created from Actions workflow", env.Description)
	assert.Equal(t, "https://new.example.com", env.ExternalURL)

	// Test getting existing environment (should return same environment)
	env2, err := CreateOrGetEnvironmentByName(db.DefaultContext, repoID, "new-env", userID+1, "https://different.example.com")
	require.NoError(t, err)
	assert.Equal(t, env.ID, env2.ID)
	assert.Equal(t, env.Name, env2.Name)
	assert.Equal(t, int64(userID), env2.CreatedByID) // Should keep original creator
	assert.Equal(t, "Auto-created from Actions workflow", env2.Description) // Should keep original description
	assert.Equal(t, "https://new.example.com", env2.ExternalURL) // Should keep original URL

	// Cleanup
	err = DeleteEnvironment(db.DefaultContext, repoID, "new-env")
	require.NoError(t, err)
}

func TestExtractEnvironmentsFromWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		workflow string
		expected []*WorkflowEnvironmentInfo
	}{
		{
			name: "simple string environment",
			workflow: `
jobs:
  deploy:
    environment: production
`,
			expected: []*WorkflowEnvironmentInfo{
				{JobID: "deploy", Environment: "production"},
			},
		},
		{
			name: "object environment with URL",
			workflow: `
jobs:
  deploy:
    environment:
      name: staging
      url: https://staging.example.com
`,
			expected: []*WorkflowEnvironmentInfo{
				{JobID: "deploy", Environment: "staging", URL: "https://staging.example.com"},
			},
		},
		{
			name: "multiple jobs with environments",
			workflow: `
jobs:
  test:
    runs-on: ubuntu-latest
  deploy-staging:
    environment: staging
  deploy-prod:
    environment:
      name: production
      url: https://prod.example.com
`,
			expected: []*WorkflowEnvironmentInfo{
				{JobID: "deploy-staging", Environment: "staging"},
				{JobID: "deploy-prod", Environment: "production", URL: "https://prod.example.com"},
			},
		},
		{
			name: "no environments",
			workflow: `
jobs:
  test:
    runs-on: ubuntu-latest
`,
			expected: []*WorkflowEnvironmentInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envInfos, err := ExtractEnvironmentsFromWorkflow([]byte(tt.workflow))
			require.NoError(t, err)
			
			assert.Len(t, envInfos, len(tt.expected))
			
			for i, expected := range tt.expected {
				if i < len(envInfos) {
					assert.Equal(t, expected.JobID, envInfos[i].JobID)
					assert.Equal(t, expected.Environment, envInfos[i].Environment)
					assert.Equal(t, expected.URL, envInfos[i].URL)
				}
			}
		})
	}
}
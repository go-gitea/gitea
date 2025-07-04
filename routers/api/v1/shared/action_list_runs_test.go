// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

// TestListRunsWorkflowFiltering tests that ListRuns properly handles
// the workflow_id path parameter for filtering runs by workflow.
func TestListRunsWorkflowFiltering(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadUser(t, ctx, 2)

	// Test case 1: With workflow_id parameter (simulating /workflows/{workflow_id}/runs endpoint)
	ctx.SetPathParam("workflow_id", "test-workflow-123")

	opts, err := buildRunOptions(ctx, 0, ctx.Repo.Repository.ID)
	assert.NoError(t, err)

	// Verify the WorkflowID is correctly extracted from path parameter
	assert.Equal(t, "test-workflow-123", opts.WorkflowID)
	assert.Equal(t, ctx.Repo.Repository.ID, opts.RepoID)
	assert.Equal(t, int64(0), opts.OwnerID)

	// Test case 2: Without workflow_id parameter (general /runs endpoint)
	ctx2, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx2, 1)
	contexttest.LoadUser(t, ctx2, 2)
	// No SetPathParam call - simulates general runs endpoint

	opts2, err := buildRunOptions(ctx2, 0, ctx2.Repo.Repository.ID)
	assert.NoError(t, err)

	// Verify WorkflowID is empty when path parameter is not set
	assert.Empty(t, opts2.WorkflowID)
	assert.Equal(t, ctx2.Repo.Repository.ID, opts2.RepoID)
}

// Tests for new query parameters

// TestListRunsExcludePullRequestsParam tests that ListRuns properly handles
// the exclude_pull_requests parameter.
func TestListRunsExcludePullRequestsParam(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// Test case 1: With exclude_pull_requests=true
	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1?exclude_pull_requests=true")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadUser(t, ctx, 2)

	// Call the actual production logic
	opts, err := buildRunOptions(ctx, 0, ctx.Repo.Repository.ID)
	assert.NoError(t, err)

	// Verify the ExcludePullRequests is correctly set based on the form value
	assert.True(t, opts.ExcludePullRequests)

	// Test case 2: With exclude_pull_requests=1
	ctx2, _ := contexttest.MockAPIContext(t, "user2/repo1?exclude_pull_requests=1")
	contexttest.LoadRepo(t, ctx2, 1)
	contexttest.LoadUser(t, ctx2, 2)

	opts2, err := buildRunOptions(ctx2, 0, ctx2.Repo.Repository.ID)
	assert.NoError(t, err)

	// Verify the ExcludePullRequests is correctly set for "1" value
	assert.True(t, opts2.ExcludePullRequests)

	// Test case 3: With exclude_pull_requests=false (should not set the flag)
	ctx3, _ := contexttest.MockAPIContext(t, "user2/repo1?exclude_pull_requests=false")
	contexttest.LoadRepo(t, ctx3, 1)
	contexttest.LoadUser(t, ctx3, 2)

	opts3, err := buildRunOptions(ctx3, 0, ctx3.Repo.Repository.ID)
	assert.NoError(t, err)

	// Verify the ExcludePullRequests is NOT set for "false" value
	assert.False(t, opts3.ExcludePullRequests)
}

// TestListRunsCreatedParam tests that ListRuns properly handles
// the created parameter for date filtering.
func TestListRunsCreatedParam(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// Test case 1: With created in date range format
	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1?created=2023-01-01..2023-12-31")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadUser(t, ctx, 2)

	opts, err := buildRunOptions(ctx, 0, ctx.Repo.Repository.ID)
	assert.NoError(t, err)

	// Verify the date range is correctly parsed
	expectedStart, _ := time.Parse("2006-01-02", "2023-01-01")
	expectedEnd, _ := time.Parse("2006-01-02", "2023-12-31")
	expectedEnd = expectedEnd.Add(24*time.Hour - time.Second)

	assert.Equal(t, expectedStart, opts.CreatedAfter)
	assert.Equal(t, expectedEnd, opts.CreatedBefore)

	// Test case 2: With created in ">=" format
	ctx2, _ := contexttest.MockAPIContext(t, "user2/repo1?created=>=2023-01-01")
	contexttest.LoadRepo(t, ctx2, 1)
	contexttest.LoadUser(t, ctx2, 2)

	opts2, err := buildRunOptions(ctx2, 0, ctx2.Repo.Repository.ID)
	assert.NoError(t, err)

	// Verify the date is correctly parsed
	expectedStart2, _ := time.Parse("2006-01-02", "2023-01-01")
	assert.Equal(t, expectedStart2, opts2.CreatedAfter)
	assert.True(t, opts2.CreatedBefore.IsZero())

	// Test case 3: With created in exact date format
	ctx3, _ := contexttest.MockAPIContext(t, "user2/repo1?created=2023-06-15")
	contexttest.LoadRepo(t, ctx3, 1)
	contexttest.LoadUser(t, ctx3, 2)

	opts3, err := buildRunOptions(ctx3, 0, ctx3.Repo.Repository.ID)
	assert.NoError(t, err)

	// Verify the exact date is correctly parsed to a date range
	exactDate, _ := time.Parse("2006-01-02", "2023-06-15")
	assert.Equal(t, exactDate, opts3.CreatedAfter)
	assert.Equal(t, exactDate.Add(24*time.Hour-time.Second), opts3.CreatedBefore)
}

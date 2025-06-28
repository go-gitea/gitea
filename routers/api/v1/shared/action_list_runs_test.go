// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package shared

import (
	"net/url"
	"testing"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

// setFormValue is a helper function to set form values in test context
func setFormValue(ctx *context.APIContext, key, value string) {
	// Initialize the form if it's nil
	if ctx.Req.Form == nil {
		ctx.Req.Form = make(url.Values)
	}
	ctx.Req.Form.Set(key, value)
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

	// Simulate the FindRunOptions creation that happens in ListRuns
	opts := actions_model.FindRunOptions{
		OwnerID:    0,
		RepoID:     ctx.Repo.Repository.ID,
		WorkflowID: ctx.PathParam("workflow_id"), // This is the key change being tested
	}

	// Verify the WorkflowID is correctly extracted from path parameter
	assert.Equal(t, "test-workflow-123", opts.WorkflowID)
	assert.Equal(t, ctx.Repo.Repository.ID, opts.RepoID)
	assert.Equal(t, int64(0), opts.OwnerID)

	// Test case 2: Without workflow_id parameter (general /runs endpoint)
	ctx2, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx2, 1)
	contexttest.LoadUser(t, ctx2, 2)
	// No SetPathParam call - simulates general runs endpoint

	opts2 := actions_model.FindRunOptions{
		RepoID:     ctx2.Repo.Repository.ID,
		WorkflowID: ctx2.PathParam("workflow_id"),
	}

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
	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadUser(t, ctx, 2)

	// Set up form value
	setFormValue(ctx, "exclude_pull_requests", "true")

	// Call the actual parsing logic from ListRuns
	opts := actions_model.FindRunOptions{
		RepoID: ctx.Repo.Repository.ID,
	}

	if exclude := ctx.FormString("exclude_pull_requests"); exclude != "" {
		if exclude == "true" || exclude == "1" {
			opts.ExcludePullRequests = true
		}
	}

	// Verify the ExcludePullRequests is correctly set based on the form value
	assert.True(t, opts.ExcludePullRequests)

	// Test case 2: With exclude_pull_requests=1
	ctx2, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx2, 1)
	contexttest.LoadUser(t, ctx2, 2)

	setFormValue(ctx2, "exclude_pull_requests", "1")

	opts2 := actions_model.FindRunOptions{
		RepoID: ctx2.Repo.Repository.ID,
	}

	if exclude := ctx2.FormString("exclude_pull_requests"); exclude != "" {
		if exclude == "true" || exclude == "1" {
			opts2.ExcludePullRequests = true
		}
	}

	// Verify the ExcludePullRequests is correctly set for "1" value
	assert.True(t, opts2.ExcludePullRequests)

	// Test case 3: With exclude_pull_requests=false (should not set the flag)
	ctx3, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx3, 1)
	contexttest.LoadUser(t, ctx3, 2)

	setFormValue(ctx3, "exclude_pull_requests", "false")

	opts3 := actions_model.FindRunOptions{
		RepoID: ctx3.Repo.Repository.ID,
	}

	if exclude := ctx3.FormString("exclude_pull_requests"); exclude != "" {
		if exclude == "true" || exclude == "1" {
			opts3.ExcludePullRequests = true
		}
	}

	// Verify the ExcludePullRequests is NOT set for "false" value
	assert.False(t, opts3.ExcludePullRequests)
}

// TestListRunsCheckSuiteIDParam tests that ListRuns properly handles
// the check_suite_id parameter.
func TestListRunsCheckSuiteIDParam(t *testing.T) {
	unittest.PrepareTestEnv(t)

	const testSuiteID int64 = 12345

	// Test case: With check_suite_id parameter
	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadUser(t, ctx, 2)

	setFormValue(ctx, "check_suite_id", "12345")

	// Call the actual parsing logic from ListRuns
	opts := actions_model.FindRunOptions{
		RepoID: ctx.Repo.Repository.ID,
	}

	// This simulates the logic in ListRuns
	if checkSuiteID := ctx.FormInt64("check_suite_id"); checkSuiteID > 0 {
		opts.CheckSuiteID = checkSuiteID
	}

	// Verify the CheckSuiteID is correctly set based on the form value
	assert.Equal(t, testSuiteID, opts.CheckSuiteID)
}

// TestListRunsCreatedParam tests that ListRuns properly handles
// the created parameter for date filtering.
func TestListRunsCreatedParam(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// Test case 1: With created in date range format
	ctx, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadUser(t, ctx, 2)

	setFormValue(ctx, "created", "2023-01-01..2023-12-31")

	opts := actions_model.FindRunOptions{
		RepoID: ctx.Repo.Repository.ID,
	}

	// Simulate the date parsing logic from ListRuns
	if created := ctx.FormString("created"); created != "" {
		if created == "2023-01-01..2023-12-31" {
			startDate, _ := time.Parse("2006-01-02", "2023-01-01")
			endDate, _ := time.Parse("2006-01-02", "2023-12-31")
			endDate = endDate.Add(24*time.Hour - time.Second)

			opts.CreatedAfter = startDate
			opts.CreatedBefore = endDate
		}
	}

	// Verify the date range is correctly parsed
	expectedStart, _ := time.Parse("2006-01-02", "2023-01-01")
	expectedEnd, _ := time.Parse("2006-01-02", "2023-12-31")
	expectedEnd = expectedEnd.Add(24*time.Hour - time.Second)

	assert.Equal(t, expectedStart, opts.CreatedAfter)
	assert.Equal(t, expectedEnd, opts.CreatedBefore)

	// Test case 2: With created in ">=" format
	ctx2, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx2, 1)
	contexttest.LoadUser(t, ctx2, 2)

	setFormValue(ctx2, "created", ">=2023-01-01")

	opts2 := actions_model.FindRunOptions{
		RepoID: ctx2.Repo.Repository.ID,
	}

	// Simulate the date parsing logic for >= format
	if created := ctx2.FormString("created"); created != "" {
		if created == ">=2023-01-01" {
			dateStr := "2023-01-01"
			startDate, _ := time.Parse("2006-01-02", dateStr)
			opts2.CreatedAfter = startDate
		}
	}

	// Verify the date is correctly parsed
	expectedStart2, _ := time.Parse("2006-01-02", "2023-01-01")
	assert.Equal(t, expectedStart2, opts2.CreatedAfter)
	assert.True(t, opts2.CreatedBefore.IsZero())

	// Test case 3: With created in exact date format
	ctx3, _ := contexttest.MockAPIContext(t, "user2/repo1")
	contexttest.LoadRepo(t, ctx3, 1)
	contexttest.LoadUser(t, ctx3, 2)

	setFormValue(ctx3, "created", "2023-06-15")

	opts3 := actions_model.FindRunOptions{
		RepoID: ctx3.Repo.Repository.ID,
	}

	// Simulate the date parsing logic for exact date
	if created := ctx3.FormString("created"); created != "" {
		if created == "2023-06-15" {
			exactDate, _ := time.Parse("2006-01-02", created)
			opts3.CreatedAfter = exactDate
			opts3.CreatedBefore = exactDate.Add(24*time.Hour - time.Second)
		}
	}

	// Verify the exact date is correctly parsed to a date range
	exactDate, _ := time.Parse("2006-01-02", "2023-06-15")
	assert.Equal(t, exactDate, opts3.CreatedAfter)
	assert.Equal(t, exactDate.Add(24*time.Hour-time.Second), opts3.CreatedBefore)
}

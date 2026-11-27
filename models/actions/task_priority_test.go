// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strings"
	"testing"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobQueueKey(t *testing.T) {
	job := &ActionRunJob{Updated: 10_000}
	assert.EqualValues(t, 10_000, job.jobQueueKey(1800))

	job.Priority = jobparser.PriorityHigh
	assert.EqualValues(t, 10_000-1800, job.jobQueueKey(1800))
	assert.EqualValues(t, 10_000, job.jobQueueKey(0), "a zero step ignores priorities")

	job.Priority = jobparser.PriorityLow
	assert.EqualValues(t, 10_000+1800, job.jobQueueKey(1800))

	assert.Equal(t, "updated", jobQueueKeyExpr(0))
	assert.Equal(t, "(updated - priority * 1800)", jobQueueKeyExpr(1800))
}

func TestCreateTaskForRunnerPriority(t *testing.T) {
	const label = "priority-test"
	base := timeutil.TimeStamp(1_700_000_000)

	type queued struct {
		name     string
		priority int
		queuedAt time.Duration // after base
	}
	for _, tt := range []struct {
		name      string
		aging     time.Duration
		batchSize int
		jobs      []queued
		wantOrder []string
	}{
		{
			name:  "high overtakes earlier low and normal",
			aging: 30 * time.Minute,
			jobs: []queued{
				{"low-1", jobparser.PriorityLow, 0},
				{"low-2", jobparser.PriorityLow, time.Second},
				{"normal", jobparser.PriorityNormal, 2 * time.Second},
				{"high", jobparser.PriorityHigh, 3 * time.Second},
			},
			wantOrder: []string{"high", "normal", "low-1", "low-2"},
		},
		{
			name:  "aging lets a long-waiting low job go before a fresh high job",
			aging: 30 * time.Minute,
			jobs: []queued{
				{"old-low", jobparser.PriorityLow, 0},
				{"fresh-high", jobparser.PriorityHigh, 61 * time.Minute}, // more than two steps later
				{"recent-high", jobparser.PriorityHigh, 59 * time.Minute},
			},
			wantOrder: []string{"recent-high", "old-low", "fresh-high"},
		},
		{
			name:  "equal priority keeps queue order",
			aging: 30 * time.Minute,
			jobs: []queued{
				{"b", jobparser.PriorityHigh, time.Second},
				{"a", jobparser.PriorityHigh, 0},
				{"c", jobparser.PriorityHigh, time.Second},
			},
			wantOrder: []string{"a", "b", "c"},
		},
		{
			name:  "zero aging ignores priorities",
			aging: 0,
			jobs: []queued{
				{"low", jobparser.PriorityLow, 0},
				{"high", jobparser.PriorityHigh, time.Second},
			},
			wantOrder: []string{"low", "high"},
		},
		{
			name:      "pagination follows priority order",
			aging:     30 * time.Minute,
			batchSize: 2,
			jobs: []queued{
				{"low-1", jobparser.PriorityLow, 0},
				{"low-2", jobparser.PriorityLow, time.Second},
				{"low-3", jobparser.PriorityLow, 2 * time.Second},
				{"normal", jobparser.PriorityNormal, 3 * time.Second},
				{"high-1", jobparser.PriorityHigh, 4 * time.Second},
				{"high-2", jobparser.PriorityHigh, 5 * time.Second},
			},
			wantOrder: []string{"high-1", "high-2", "normal", "low-1", "low-2", "low-3"},
		},
		{
			name:      "pagination cursor follows the queue key past a page the runner cannot run",
			aging:     30 * time.Minute,
			batchSize: 2,
			jobs: []queued{
				{"low", jobparser.PriorityLow, 0},
				{"normal", jobparser.PriorityNormal, 5 * time.Second},
				{"other-high-1", jobparser.PriorityHigh, 10 * time.Second},
				{"other-high-2", jobparser.PriorityHigh, 11 * time.Second},
			},
			wantOrder: []string{"normal", "low"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			defer test.MockVariableValue(&setting.Actions.JobPriorityAging, tt.aging)()
			if tt.batchSize != 0 {
				defer test.MockVariableValue(&pickTaskBatchSize, tt.batchSize)()
			}

			run := &ActionRun{
				Title:         "priority-test-run",
				RepoID:        1,
				OwnerID:       2,
				WorkflowID:    "priority.yaml",
				Index:         9950,
				TriggerUserID: 2,
				Ref:           "refs/heads/main",
				CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
				Event:         "push",
				TriggerEvent:  "push",
				Status:        StatusWaiting,
			}
			require.NoError(t, db.Insert(t.Context(), run))
			require.NoError(t, db.Insert(t.Context(), &repo_model.RepoUnit{RepoID: run.RepoID, Type: unit.TypeActions}))

			for _, q := range tt.jobs {
				runsOn := util.Iif(strings.HasPrefix(q.name, "other-"), "other-label", label) // a label the runner does not have
				job := &ActionRunJob{
					RunID:           run.ID,
					RepoID:          run.RepoID,
					OwnerID:         run.OwnerID,
					CommitSHA:       run.CommitSHA,
					Name:            q.name,
					Attempt:         1,
					JobID:           q.name,
					Status:          StatusWaiting,
					RunsOn:          []string{runsOn},
					Priority:        q.priority,
					WorkflowPayload: []byte("on: push\njobs:\n  " + q.name + ":\n    runs-on: " + runsOn + "\n    steps:\n      - run: echo hi\n"),
					Created:         base,
					Updated:         base.AddDuration(q.queuedAt),
				}
				_, err := db.GetEngine(t.Context()).NoAutoTime().Insert(job)
				require.NoError(t, err)
			}

			runner := &ActionRunner{UUID: "priority-runner-uuid", Name: "priority-runner", AgentLabels: []string{label}}
			runner.GenerateAndFillToken()
			require.NoError(t, db.Insert(t.Context(), runner))

			var gotOrder []string
			for range tt.wantOrder {
				task, _, err := CreateTaskForRunner(t.Context(), runner)
				require.NoError(t, err)
				require.NotNil(t, task)
				gotOrder = append(gotOrder, task.Job.Name)
			}
			assert.Equal(t, tt.wantOrder, gotOrder)

			task, _, err := CreateTaskForRunner(t.Context(), runner)
			require.NoError(t, err)
			assert.Nil(t, task, "every job the runner can run is claimed")
		})
	}
}

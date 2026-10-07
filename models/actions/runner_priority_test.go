// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strconv"
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTaskForRunnerPriority(t *testing.T) {
	label := "prio-test"
	newRunner := func(t *testing.T, name string, priority int64, mod func(*ActionRunner)) *ActionRunner {
		r := &ActionRunner{
			UUID:        "prio-" + name + "-uuid",
			Name:        name,
			TokenHash:   "prio-" + name + "-hash",
			Priority:    priority,
			AgentLabels: []string{label},
			LastOnline:  timeutil.TimeStampNow(),
		}
		if mod != nil {
			mod(r)
		}
		require.NoError(t, db.Insert(t.Context(), r))
		return r
	}
	newJob := func(t *testing.T, n int) {
		run := &ActionRun{
			Title: "prio-run-" + strconv.Itoa(n), RepoID: 1, OwnerID: 2, WorkflowID: "test.yaml", Index: int64(9950 + n),
			TriggerUserID: 2, Ref: "refs/heads/main", CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
			Event: "push", TriggerEvent: "push", Status: StatusWaiting,
		}
		require.NoError(t, db.Insert(t.Context(), run))
		require.NoError(t, db.Insert(t.Context(), &ActionRunJob{
			RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, CommitSHA: run.CommitSHA, Name: "prio-job", Attempt: 1,
			JobID: "prio-job", Status: StatusWaiting, RunsOn: []string{label},
			WorkflowPayload: []byte("on: push\njobs:\n  prio-job:\n    runs-on: " + label + "\n    steps:\n      - run: echo hi\n"),
		}))
	}

	cases := []struct {
		name         string
		grace        time.Duration
		fast         func(*ActionRunner)
		wantDeferred bool
	}{
		{name: "off", grace: 0},
		{name: "preferred runner online", grace: time.Hour, wantDeferred: true},
		{name: "grace elapsed", grace: time.Nanosecond},
		{name: "preferred runner offline", grace: time.Hour, fast: func(r *ActionRunner) { r.LastOnline = 1 }},
		{name: "preferred runner disabled", grace: time.Hour, fast: func(r *ActionRunner) { r.IsDisabled = true }},
		{name: "preferred runner lacks the label", grace: time.Hour, fast: func(r *ActionRunner) { r.AgentLabels = []string{"other"} }},
		{name: "preferred runner of another repo", grace: time.Hour, fast: func(r *ActionRunner) { r.RepoID = 2 }},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			defer func(orig time.Duration) { setting.Actions.PreferredRunnerGrace = orig }(setting.Actions.PreferredRunnerGrace)
			setting.Actions.PreferredRunnerGrace = c.grace

			slow := newRunner(t, "slow", 0, nil)
			fast := newRunner(t, "fast", 10, c.fast)
			newJob(t, i)

			task, ok, deferred, err := CreateTaskForRunnerWithDeferral(t.Context(), slow)
			require.NoError(t, err)
			assert.Equal(t, c.wantDeferred, deferred)
			assert.Equal(t, !c.wantDeferred, ok)
			if ok {
				assert.Equal(t, slow.ID, task.RunnerID)
				return
			}

			// the preferred runner takes it, and is never deferred itself
			task, ok, deferred, err = CreateTaskForRunnerWithDeferral(t.Context(), fast)
			require.NoError(t, err)
			assert.True(t, ok)
			assert.False(t, deferred)
			assert.Equal(t, fast.ID, task.RunnerID)
		})
	}
}

func TestFindRunnerOptionsSortByPriority(t *testing.T) {
	assert.Equal(t, "priority ASC, id ASC", FindRunnerOptions{Sort: "lowestpriority"}.ToOrders())
	assert.Equal(t, "priority DESC, id ASC", FindRunnerOptions{Sort: "highestpriority"}.ToOrders())
}

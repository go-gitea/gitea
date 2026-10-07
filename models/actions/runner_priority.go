// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"xorm.io/builder"
)

// runnerDeferral decides whether a waiting job should be left to a runner with a higher priority.
type runnerDeferral struct {
	runner    *ActionRunner
	loaded    bool
	preferred []*ActionRunner
}

func newRunnerDeferral(runner *ActionRunner) *runnerDeferral {
	return &runnerDeferral{runner: runner}
}

// deferred reports whether job has waited less than the grace period and an online, enabled runner with a
// higher priority could run it. A failure to look up the runners never defers.
func (d *runnerDeferral) deferred(ctx context.Context, job *ActionRunJob) bool {
	grace := setting.Actions.PreferredRunnerGrace
	if grace <= 0 || time.Since(job.Updated.AsTime()) >= grace {
		return false
	}
	if !d.loaded {
		d.loaded = true
		if err := db.GetEngine(ctx).
			Where(builder.Gt{"priority": d.runner.Priority}).
			And(builder.Eq{"is_disabled": false}).
			And(builder.Gt{"last_online": time.Now().Add(-RunnerOfflineTime).Unix()}).
			Find(&d.preferred); err != nil {
			log.Error("load runners preferred over %d: %v", d.runner.ID, err)
			d.preferred = nil
		}
	}
	for _, r := range d.preferred {
		if r.availableForJob(job) && r.CanMatchLabels(job.RunsOn) {
			return true
		}
	}
	return false
}

// availableForJob reports whether the runner's scope (global, owner or repository) covers the job's repository.
func (r *ActionRunner) availableForJob(job *ActionRunJob) bool {
	switch {
	case r.RepoID != 0:
		return r.RepoID == job.RepoID
	case r.OwnerID != 0:
		return r.OwnerID == job.OwnerID
	default:
		return true
	}
}

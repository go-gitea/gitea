// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"maps"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/services/convert"

	"xorm.io/builder"
)

// StartScheduleTasks start the task
func StartScheduleTasks(ctx context.Context) error {
	return startTasks(ctx)
}

// startTasks starts every due spec and returns an error if any of them failed.
func startTasks(ctx context.Context) error {
	var failed int
	now := time.Now()
	err := db.Iterate(ctx,
		builder.And(builder.Gt{"next": 0}, builder.Lte{"next": now.Unix()}),
		func(ctx context.Context, row *actions_model.ActionScheduleSpec) error {
			// one failing spec must not abort the pass, or a single broken workflow stops every other schedule
			if err := startTask(ctx, row, now); err != nil {
				failed++
				log.Error("start schedule spec %d (repo %d, schedule %d): %v", row.ID, row.RepoID, row.ScheduleID, err)
			}
			return nil
		})
	if err != nil {
		return fmt.Errorf("iterate specs: %w", err)
	}

	// surfaces as an admin notice through the cron task, once per occurrence rather than once per pass
	if failed > 0 {
		return fmt.Errorf("%d schedule(s) could not be started", failed)
	}
	return nil
}

// startTask advances the spec to its next occurrence before creating the run, so a failing workflow
// retries on its own schedule instead of on every pass, and a failed update cannot duplicate the run.
func startTask(ctx context.Context, row *actions_model.ActionScheduleSpec, now time.Time) error {
	cronSchedule, err := row.Parse()
	if err != nil {
		return fmt.Errorf("parse %q: %w", row.Spec, err)
	}
	row.Prev = row.Next
	row.Next = timeutil.TimeStamp(cronSchedule.Next(now.Add(time.Minute)).Unix())
	if err := actions_model.UpdateScheduleSpec(ctx, row, "prev", "next"); err != nil {
		return fmt.Errorf("update spec: %w", err)
	}

	// a spec whose schedule or repo row is gone is skipped, not reported on every occurrence
	schedule, exist, err := db.GetByID[actions_model.ActionSchedule](ctx, row.ScheduleID)
	if err != nil {
		return fmt.Errorf("get schedule %d: %w", row.ScheduleID, err)
	} else if !exist {
		return nil
	}
	repo, exist, err := db.GetByID[repo_model.Repository](ctx, row.RepoID)
	if err != nil {
		return fmt.Errorf("get repo %d: %w", row.RepoID, err)
	} else if !exist {
		return nil
	}
	row.Schedule, row.Repo = schedule, repo

	// only archived repos are skipped; mirrors keep their schedules because a mirror is a normal repo
	// for Actions, and nightly builds or scans of the mirrored code are a common reason to run one
	if row.Repo.IsArchived {
		return nil
	}

	cfg, err := row.Repo.GetUnit(ctx, unit.TypeActions)
	if err != nil {
		if repo_model.IsErrUnitTypeNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetUnit: %w", err)
	}
	if cfg.ActionsConfig().IsWorkflowDisabled(row.Schedule.WorkflowID) {
		return nil
	}

	if err := CreateScheduleTaskBySpec(ctx, row); err != nil {
		return fmt.Errorf("create run for %s workflow %q: %w", row.Repo.FullName(), row.Schedule.WorkflowID, err)
	}
	return nil
}

// CreateScheduleTaskBySpec creates a scheduled task from a cron action schedule spec.
// It creates an action run based on the schedule, inserts it into the database, and creates commit statuses for each job.
func CreateScheduleTaskBySpec(ctx context.Context, spec *actions_model.ActionScheduleSpec) error {
	cron := spec.Schedule

	// Scheduled runs carry no webhook payload; synthesize what github.event.* expects.
	if err := spec.Repo.LoadOwner(ctx); err != nil {
		return fmt.Errorf("LoadOwner: %w", err)
	}
	fields := map[string]any{
		"repository": convert.ToRepo(ctx, spec.Repo, access_model.Permission{AccessMode: perm_model.AccessModeRead}),
		"sender":     convert.ToUser(ctx, user_model.NewActionsUser(), nil),
	}
	if spec.Repo.Owner.IsOrganization() {
		fields["organization"] = convert.ToOrganization(ctx, organization.OrgFromUser(spec.Repo.Owner))
	}
	eventPayload := withScheduleInEventPayload(cron.EventPayload, spec.Spec, fields)

	// Create a new action run based on the schedule
	run := &actions_model.ActionRun{
		Title:         cron.Title,
		RepoID:        cron.RepoID,
		OwnerID:       cron.OwnerID,
		WorkflowID:    cron.WorkflowID,
		TriggerUserID: cron.TriggerUserID,
		Ref:           cron.Ref,
		CommitSHA:     cron.CommitSHA,
		Event:         cron.Event,
		EventPayload:  eventPayload,
		TriggerEvent:  string(webhook_module.HookEventSchedule),
		ScheduleID:    cron.ID,
		Status:        actions_model.StatusWaiting,
		// schedule runs the repo's own workflow at the recorded commit
		WorkflowRepoID:    cron.RepoID,
		WorkflowCommitSHA: cron.CommitSHA,
	}

	// FIXME cron.Content might be outdated if the workflow file has been changed.
	// Load the latest sha from default branch
	// Insert the action run and its associated jobs into the database
	if err := PrepareRunAndInsert(ctx, cron.Content, run, nil); err != nil {
		return err
	}

	// Return nil if no errors occurred
	return nil
}

func withScheduleInEventPayload(eventPayload, schedule string, fields map[string]any) string {
	if schedule == "" {
		return eventPayload
	}

	// eventPayload originates from json.Marshal(input.Payload) in handleSchedules,
	// so a nil payload is stored as the literal "null" and pre-existing rows may be
	// empty. Both cases start from a fresh map so the schedule field can still be set.
	var event map[string]any
	if eventPayload != "" {
		if err := json.Unmarshal([]byte(eventPayload), &event); err != nil {
			log.Error("withScheduleInEventPayload: unmarshal: %v", err)
			return eventPayload
		}
	}
	if event == nil {
		event = map[string]any{}
	}

	maps.Copy(event, fields)
	event["schedule"] = schedule
	updatedPayload, err := json.Marshal(event)
	if err != nil {
		log.Error("withScheduleInEventPayload: marshal: %v", err)
		return eventPayload
	}

	return string(updatedPayload)
}

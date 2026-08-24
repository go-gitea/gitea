// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
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
)

// StartScheduleTasks start the task
func StartScheduleTasks(ctx context.Context) error {
	return startTasks(ctx)
}

// startTasks retrieves specifications in pages, creates a schedule task for each specification,
// and updates the specification's next run time and previous run time.
// The function returns an error if there's an issue with finding or updating the specifications.
func startTasks(ctx context.Context) error {
	// Set the page size
	pageSize := 50

	// Retrieve specs in pages until all specs have been retrieved
	now := time.Now()
	for page := 1; ; page++ {
		// Retrieve the specs for the current page
		specs, _, err := actions_model.FindSpecs(ctx, actions_model.FindSpecOptions{
			ListOptions: db.ListOptions{
				Page:     page,
				PageSize: pageSize,
			},
			Next: now.Unix(),
		})
		if err != nil {
			return fmt.Errorf("find specs: %w", err)
		}

		if err := specs.LoadRepos(ctx); err != nil {
			return fmt.Errorf("LoadRepos: %w", err)
		}

		// Loop through each spec and create a schedule task for it.
		// One failing spec must not abort the pass, otherwise a single broken workflow stops every other schedule.
		for _, row := range specs {
			if err := startTask(ctx, row, now); err != nil {
				log.Error("start schedule spec %d (repo %d, schedule %d): %v", row.ID, row.RepoID, row.ScheduleID, err)
			}
		}

		// Stop if all specs have been retrieved
		if len(specs) < pageSize {
			break
		}
	}

	return nil
}

// startTask creates the run for one due spec and moves the spec to its next occurrence.
// The spec is always advanced, even when the run could not be created, so a workflow that
// keeps failing is retried on its own schedule instead of on every pass.
func startTask(ctx context.Context, row *actions_model.ActionScheduleSpec, now time.Time) error {
	if row.Repo == nil || row.Schedule == nil || row.Repo.IsArchived {
		return nil
	}

	cfg, err := row.Repo.GetUnit(ctx, unit.TypeActions)
	if err != nil {
		if repo_model.IsErrUnitTypeNotExist(err) {
			// Skip if the actions unit of this repo is disabled.
			return nil
		}
		return fmt.Errorf("GetUnit: %w", err)
	}
	if cfg.ActionsConfig().IsWorkflowDisabled(row.Schedule.WorkflowID) {
		return nil
	}

	createErr := CreateScheduleTask(ctx, row)
	if createErr != nil {
		createErr = fmt.Errorf("CreateScheduleTask: %w", createErr)
	}

	schedule, err := row.Parse()
	if err != nil {
		return errors.Join(createErr, fmt.Errorf("Parse: %w", err))
	}

	// Update the spec's next run time and previous run time
	row.Prev = row.Next
	row.Next = timeutil.TimeStamp(schedule.Next(now.Add(1 * time.Minute)).Unix())
	if err := actions_model.UpdateScheduleSpec(ctx, row, "prev", "next"); err != nil {
		return errors.Join(createErr, fmt.Errorf("UpdateScheduleSpec: %w", err))
	}

	return createErr
}

// CreateScheduleTask creates a scheduled task from a cron action schedule spec.
// It creates an action run based on the schedule, inserts it into the database, and creates commit statuses for each job.
func CreateScheduleTask(ctx context.Context, spec *actions_model.ActionScheduleSpec) error {
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

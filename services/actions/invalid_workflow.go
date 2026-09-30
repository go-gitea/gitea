// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unit"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

func handleInvalidWorkflows(ctx context.Context, input *notifyInput, ref git.RefName, commit *git.Commit, invalid map[string]error) {
	if len(invalid) == 0 {
		return
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		log.Error("marshal event payload: %v", err)
		return
	}
	actionsConfig := input.Repo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
	for entryName, parseErr := range invalid {
		if actionsConfig.IsWorkflowDisabled(entryName) {
			continue
		}
		now := timeutil.TimeStampNow()
		run := &actions_model.ActionRun{
			Title: util.EllipsisDisplayString(commit.MessageTitle(), 255), RepoID: input.Repo.ID, Repo: input.Repo, OwnerID: input.Repo.OwnerID,
			WorkflowID: entryName, TriggerUserID: input.Doer.ID, TriggerUser: input.Doer, Ref: ref.String(),
			CommitSHA: commit.ID.String(), Event: input.Event, TriggerEvent: string(input.Event), EventPayload: string(payload),
			WorkflowRepoID: input.Repo.ID, WorkflowCommitSHA: commit.ID.String(), Status: actions_model.StatusFailure, Started: now, Stopped: now,
		}
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			if run.Index, err = db.GetNextResourceIndex(ctx, "action_run_index", run.RepoID); err != nil {
				return err
			}
			if err := db.Insert(ctx, run); err != nil {
				return err
			}
			attempt := &actions_model.ActionRunAttempt{RepoID: run.RepoID, RunID: run.ID, Attempt: 1, TriggerUserID: run.TriggerUserID, Status: run.Status, Started: now, Stopped: now}
			if err := db.Insert(ctx, attempt); err != nil {
				return err
			}
			run.LatestAttemptID = attempt.ID
			if err := actions_model.UpdateRun(ctx, run, "latest_attempt_id"); err != nil {
				return err
			}
			content := fmt.Sprintf("**Invalid workflow file: %s**\n\n```\n%v\n```\n", entryName, parseErr)
			return db.Insert(ctx, &actions_model.ActionRunJobSummary{
				RepoID: run.RepoID, RunID: run.ID, RunAttemptID: attempt.ID, Content: content, ContentSize: int64(len(content)), ContentType: actions_model.JobSummaryContentTypeMarkdown,
			})
		}); err != nil {
			log.Error("insert run for invalid workflow %q: %v", entryName, err)
			continue
		}
		if err := createWorkflowCommitStatus(ctx, run.Repo, run.CommitSHA, entryName+" ("+run.TriggerEvent+")", run.WorkflowID,
			commitstatus.CommitStatusFailure, run.Link(), "Invalid workflow file", false); err != nil {
			log.Error("create commit status for invalid workflow %q: %v", entryName, err)
		}
		NotifyWorkflowRunStatusUpdate(ctx, run)
	}
}

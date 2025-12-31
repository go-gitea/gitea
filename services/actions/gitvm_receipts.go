// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	gitvm_ledger "code.gitea.io/gitea/modules/gitvm/ledger"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// emitCIRunStartReceipt emits a ci.run.start receipt.
// IMPORTANT: must never block or fail the primary action path.
func emitCIRunStartReceipt(ctx context.Context, run *actions_model.ActionRun) {
	if !setting.GitVM.Enabled {
		return
	}

	receipt, err := buildCIRunStartReceipt(ctx, run)
	if err != nil {
		log.Warn("GitVM: build receipt failed (ci.run.start): %v", err)
		return
	}

	l := gitvm_ledger.New(setting.GitVM.Dir)
	if err := l.Emit(receipt); err != nil {
		log.Warn("GitVM: emit failed (ci.run.start) for run %d: %v", run.ID, err)
	}
}

// emitCIRunEndReceipt emits a ci.run.end receipt (terminal run state).
func emitCIRunEndReceipt(ctx context.Context, run *actions_model.ActionRun) {
	if !setting.GitVM.Enabled {
		return
	}

	receipt, err := buildCIRunEndReceipt(ctx, run)
	if err != nil {
		log.Warn("GitVM: build receipt failed (ci.run.end): %v", err)
		return
	}

	l := gitvm_ledger.New(setting.GitVM.Dir)
	if err := l.Emit(receipt); err != nil {
		log.Warn("GitVM: emit failed (ci.run.end) for run %d: %v", run.ID, err)
	}
}

// buildCIRunStartReceipt constructs a ci.run.start receipt
func buildCIRunStartReceipt(ctx context.Context, run *actions_model.ActionRun) (*gitvm_ledger.Receipt, error) {
	repoRef, actorRef := resolveRepoAndActor(ctx, run.RepoID, run.TriggerUserID)

	payload := gitvm_ledger.CIRunStartPayload{
		RunID:      run.ID,
		WorkflowID: run.WorkflowID,
		CommitSHA:  run.CommitSHA,
		Ref:        run.Ref,
		Event:      string(run.Event), // webhook_module.HookEventType -> string
	}

	return &gitvm_ledger.Receipt{
		Type:     "ci.run.start",
		Repo:     repoRef,
		Actor:    actorRef,
		Payload:  payload,
		TsUnixMs: timeStampToMs(run.Created),
	}, nil
}

// buildCIRunEndReceipt constructs a ci.run.end receipt
func buildCIRunEndReceipt(ctx context.Context, run *actions_model.ActionRun) (*gitvm_ledger.Receipt, error) {
	repoRef, actorRef := resolveRepoAndActor(ctx, run.RepoID, run.TriggerUserID)

	// Duration: use (Stopped - Started) if both are set
	durationMs := computeRunDurationMs(run)

	payload := gitvm_ledger.CIRunEndPayload{
		RunID:      run.ID,
		Status:     run.Status.String(),
		DurationMs: durationMs,
		CommitSHA:  run.CommitSHA,
		WorkflowID: run.WorkflowID,
		Ref:        run.Ref,
		Event:      string(run.Event),
	}

	// Use stopped time as receipt timestamp
	ts := timeStampToMs(run.Stopped)
	if ts == 0 {
		// fallback to current time if stopped not set (shouldn't happen for terminal states)
		ts = 0 // ledger.Emit will set it
	}

	return &gitvm_ledger.Receipt{
		Type:     "ci.run.end",
		Repo:     repoRef,
		Actor:    actorRef,
		Payload:  payload,
		TsUnixMs: ts,
	}, nil
}

// resolveRepoAndActor does best-effort lookups for repo and actor names.
// Never fails the main path.
func resolveRepoAndActor(ctx context.Context, repoID, userID int64) (gitvm_ledger.RepoRef, gitvm_ledger.ActorRef) {
	repoRef := gitvm_ledger.RepoRef{ID: repoID}
	actorRef := gitvm_ledger.ActorRef{ID: userID}

	// Best-effort repo lookup
	if repoID > 0 {
		if repo, err := repo_model.GetRepositoryByID(ctx, repoID); err == nil && repo != nil {
			repoRef.Full = repo.FullName()
		}
	}

	// Best-effort user lookup
	if userID > 0 {
		if user, err := user_model.GetUserByID(ctx, userID); err == nil && user != nil {
			actorRef.Username = user.Name
		}
	}

	return repoRef, actorRef
}

// timeStampToMs converts timeutil.TimeStamp (Unix seconds) to milliseconds
func timeStampToMs(ts any) int64 {
	// timeutil.TimeStamp is int64 representing Unix seconds
	// Need to handle both int64 and timeutil.TimeStamp types
	switch v := ts.(type) {
	case int64:
		if v == 0 {
			return 0
		}
		return v * 1000
	default:
		return 0
	}
}

// computeRunDurationMs computes run duration from Started to Stopped timestamps
func computeRunDurationMs(run *actions_model.ActionRun) int64 {
	started := int64(run.Started)
	stopped := int64(run.Stopped)

	if started > 0 && stopped > 0 && stopped >= started {
		// Both are Unix seconds, convert to ms
		return (stopped - started) * 1000
	}
	return 0
}

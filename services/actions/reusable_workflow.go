// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/container"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/convert"

	"xorm.io/builder"
)

// MaxReusableCallLevels caps how deep a reusable workflow can nest:
// a top-level caller may have at most MaxReusableCallLevels nested callers below it.
const MaxReusableCallLevels = 9

// loadReusableWorkflowSource resolves the workflow file referenced by a caller's `uses:` and returns its raw bytes,
// along with the (repo_id, commit_sha) the file was loaded from.
func loadReusableWorkflowSource(ctx context.Context, run *actions_model.ActionRun, caller *actions_model.ActionRunJob, ref *jobparser.UsesRef) (content []byte, sourceRepoID int64, sourceCommitSHA string, err error) {
	if err := run.LoadAttributes(ctx); err != nil {
		return nil, 0, "", err
	}

	switch ref.Kind {
	case jobparser.UsesKindLocalSameRepo:
		// `./` is resolved against the workflow file containing the `uses:` - i.e. the caller's own source repo + commit.
		callerRepo, err := repo_model.GetRepositoryByID(ctx, caller.WorkflowSourceRepoID)
		if err != nil {
			return nil, 0, "", fmt.Errorf("look up caller source repo %d: %w", caller.WorkflowSourceRepoID, err)
		}
		bytes, resolvedSHA, err := readWorkflowFromRepo(ctx, callerRepo, caller.WorkflowSourceCommitSHA, ref.Path)
		if err != nil {
			return nil, 0, "", err
		}
		return bytes, callerRepo.ID, resolvedSHA, nil

	case jobparser.UsesKindLocalCrossRepo:
		repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ref.Owner, ref.Repo)
		if err != nil {
			return nil, 0, "", fmt.Errorf("look up cross-repo workflow source %q: %w", ref.Owner+"/"+ref.Repo, err)
		}
		ok, err := access_model.CanReadWorkflowCrossRepo(ctx, repo, run)
		if err != nil {
			return nil, 0, "", err
		}
		if !ok {
			return nil, 0, "", fmt.Errorf("no permission to read reusable workflow from %s/%s", ref.Owner, ref.Repo)
		}
		bytes, resolvedSHA, err := readWorkflowFromRepo(ctx, repo, ref.Ref, ref.Path)
		if err != nil {
			return nil, 0, "", err
		}
		return bytes, repo.ID, resolvedSHA, nil
	}
	return nil, 0, "", fmt.Errorf("unsupported uses kind %d", ref.Kind)
}

// readWorkflowFromRepo loads a workflow file from `repo` at `refOrSHA` and returns its content plus the resolved commit SHA.
func readWorkflowFromRepo(ctx context.Context, repo *repo_model.Repository, refOrSHA, path string) ([]byte, string, error) {
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return nil, "", fmt.Errorf("open repo %s: %w", repo.FullName(), err)
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(refOrSHA)
	if err != nil {
		return nil, "", fmt.Errorf("get commit %q in %s: %w", refOrSHA, repo.FullName(), err)
	}
	str, err := commit.GetFileContent(path, 1024*1024)
	if err != nil {
		return nil, "", fmt.Errorf("read %s@%s:%s: %w", repo.FullName(), refOrSHA, path, err)
	}
	return []byte(str), commit.ID.String(), nil
}

// checkCallerChain walks `caller`'s ancestor chain (via ParentJobID) and:
//   - rejects cycles (caller.CallUses appearing in any ancestor's CallUses)
//   - enforces MaxReusableCallLevels on the number of ancestors above `caller`
//
// Cycle detection is intentionally *syntactic* (string equality on CallUses), not semantic.
// So `owner/repo/lib.yml@v1` and `owner/repo/lib.yml@refs/heads/v1` resolving to the same commit are NOT treated as the same node.
// Going semantic (Owner, Repo, Path, ResolvedSHA tuples) would require extra git reads.
func checkCallerChain(ctx context.Context, caller *actions_model.ActionRunJob) error {
	if caller.ParentJobID == 0 {
		return nil // top-level caller: depth 0, no ancestors to walk
	}

	visited := make(container.Set[string])
	visited.Add(caller.CallUses)

	depth := 0
	current := caller
	for current.ParentJobID != 0 {
		next, err := actions_model.GetRunJobByRunAndID(ctx, current.RunID, current.ParentJobID)
		if err != nil {
			return fmt.Errorf("walk caller chain: %w", err)
		}
		current = next
		depth++
		if depth > MaxReusableCallLevels {
			return fmt.Errorf("reusable workflow call exceeds the maximum nesting level of %d at %q", MaxReusableCallLevels, caller.CallUses)
		}
		if current.IsReusableCaller && current.CallUses != "" {
			if visited.Contains(current.CallUses) {
				return fmt.Errorf("reusable workflow call cycle detected: %q", current.CallUses)
			}
			visited.Add(current.CallUses)
		}
	}
	return nil
}

// expandReusableWorkflowCaller loads and parses the target reusable workflow and inserts the caller's direct child jobs.
// It expands only ONE level: a child that is itself a reusable caller is inserted Blocked and expanded later by a subsequent resolver pass.
// It does NOT schedule a follow-up resolver pass; the caller of this function is responsible for emitting.
//
// All call sites (PrepareRunAndInsert, execRerunPlan, checkJobsOfCurrentRunAttempt, ApproveRuns) invoke this inside their enclosing write transaction,
// because the caller row update and the child-row inserts must commit atomically.
// Be aware this is not cheap inside a tx: it does a git read, YAML parsing, and `${{ }}` expression evaluation.
// None of the call sites is hot: each caller is expanded once per attempt.
func expandReusableWorkflowCaller(ctx context.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, caller *actions_model.ActionRunJob, vars map[string]string) error {
	// Already expanded by an earlier call, skip
	if caller.IsExpanded {
		return nil
	}

	// 1. Cycle + depth check via the ParentJobID chain.
	if err := checkCallerChain(ctx, caller); err != nil {
		return err
	}

	// 2. Parse the caller's own job (Uses, With, RawSecrets) from its WorkflowPayload.
	parsedJob, err := caller.ParseJob()
	if err != nil {
		return fmt.Errorf("parse caller job %d: %w", caller.ID, err)
	}

	// 3. Resolve `uses` and load called-workflow source.
	ref, err := ResolveUses(ctx, parsedJob.Uses)
	if err != nil {
		return fmt.Errorf("resolve uses %q: %w", parsedJob.Uses, err)
	}
	content, contentSourceRepoID, contentSourceCommitSHA, err := loadReusableWorkflowSource(ctx, run, caller, ref)
	if err != nil {
		return err
	}

	// 4. Parse the called workflow's spec (used by both secret validation and input evaluation).
	wcSpec, err := jobparser.ParseWorkflowCallSpec(content)
	if err != nil {
		return fmt.Errorf("parse called workflow spec: %w", err)
	}

	// 5. Resolve caller's `secrets:` and validate it against the callee's schema.
	inherit, secretsMap, err := jobparser.ParseCallerSecrets(parsedJob.RawSecrets)
	if err != nil {
		return fmt.Errorf("caller secrets %q: %w", caller.JobID, err)
	}
	// Under `secrets: inherit` the caller forwards all of its own secrets verbatim and does NOT name them individually,
	// so required-secret presence cannot be verified at expansion time and a missing required secret will surface at job runtime.
	// This matches GitHub Actions' behavior.
	if !inherit {
		if err := jobparser.ValidateCallerSecrets(wcSpec, secretsMap); err != nil {
			return fmt.Errorf("caller %q secrets: %w", caller.JobID, err)
		}
	}
	switch {
	case inherit:
		caller.CallSecrets = jobparser.SecretsInherit
	case len(secretsMap) > 0:
		mapBytes, err := json.Marshal(secretsMap)
		if err != nil {
			return fmt.Errorf("marshal caller secret map: %w", err)
		}
		caller.CallSecrets = string(mapBytes)
	}
	caller.ReusableWorkflowContent = content

	// 6. Evaluate caller's `with:`, then match against the callee schema.
	workflowCallInputs := map[string]any{}
	if len(wcSpec.Inputs) > 0 {
		jobResults, err := findJobNeedsAndFillJobResults(ctx, caller)
		if err != nil {
			return fmt.Errorf("find caller needs: %w", err)
		}
		parentInputs, err := getInputsForJob(ctx, run, caller)
		if err != nil {
			return err
		}
		callerGitCtx := GenerateGiteaContext(ctx, run, attempt, caller)
		evaluated, err := jobparser.EvaluateCallerWith(
			caller.JobID, parsedJob,
			callerGitCtx, jobResults, vars, parentInputs,
		)
		if err != nil {
			return fmt.Errorf("evaluate caller with: %w", err)
		}
		workflowCallInputs, err = jobparser.MatchCallerInputsAgainstSpec(wcSpec, evaluated)
		if err != nil {
			return fmt.Errorf("caller %q inputs: %w", caller.JobID, err)
		}
	}

	// 7. Build CallPayload (persisted in step 9).
	callPayload, err := (&api.WorkflowCallPayload{
		Workflow:   run.WorkflowID,
		Ref:        run.Ref,
		Repository: convert.ToRepo(ctx, run.Repo, access_model.Permission{AccessMode: perm_model.AccessModeNone}),
		Sender:     convert.ToUserWithAccessMode(ctx, run.TriggerUser, perm_model.AccessModeNone),
		Inputs:     workflowCallInputs,
	}).JSONPayload()
	if err != nil {
		return fmt.Errorf("build call payload: %w", err)
	}

	// 8. Insert direct children of this caller.
	existingChildren, err := actions_model.GetDirectChildJobsByParent(ctx, caller)
	if err != nil {
		return fmt.Errorf("get existing children of caller %d: %w", caller.ID, err)
	}
	if len(existingChildren) > 0 {
		// Should not happen - child jobs cannot be expanded before the caller gets ready
		return fmt.Errorf("invariant violation: caller %d has %d pre-existing children", caller.ID, len(existingChildren))
	}
	if err := insertCallerChildren(ctx, run, attempt, caller, content, contentSourceRepoID, contentSourceCommitSHA, vars, workflowCallInputs); err != nil {
		return err
	}

	// 9. Update caller-related cols.
	caller.CallPayload = string(callPayload)
	caller.IsExpanded = true
	n, err := actions_model.UpdateRunJob(ctx, caller,
		builder.Eq{"is_expanded": false},
		"call_secrets", "reusable_workflow_content", "call_payload", "is_expanded")
	if err != nil {
		return fmt.Errorf("commit caller %d expansion: %w", caller.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("caller %d already expanded by another writer", caller.ID)
	}
	return nil
}

// insertCallerChildren parses the called workflow with the caller's resolved inputs and inserts each parsed job.
func insertCallerChildren(ctx context.Context, run *actions_model.ActionRun, attempt *actions_model.ActionRunAttempt, caller *actions_model.ActionRunJob, content []byte, sourceRepoID int64, sourceCommitSHA string, vars map[string]string, inputs map[string]any) error {
	// Parse the called workflow with the caller's `inputs`
	gitCtx := GenerateGiteaContext(ctx, run, attempt, nil)
	if event, ok := gitCtx["event"].(map[string]any); ok {
		event["inputs"] = inputs
	}
	gitCtx["event_name"] = "workflow_call"

	childWorkflows, err := jobparser.Parse(content,
		jobparser.WithVars(vars),
		jobparser.WithGitContext(gitCtx.ToGitHubContext()),
		jobparser.WithInputs(inputs),
	)
	if err != nil {
		return fmt.Errorf("parse called workflow for caller %d: %w", caller.ID, err)
	}
	if len(childWorkflows) == 0 {
		return fmt.Errorf("called workflow for caller %d (uses %q) has no jobs", caller.ID, caller.CallUses)
	}

	priorChildren, err := actions_model.GetPriorAttemptChildrenByParent(ctx, run.ID, attempt.ID, caller.AttemptJobID)
	if err != nil {
		return fmt.Errorf("lookup prior-attempt children of caller %d: %w", caller.ID, err)
	}

	for _, sw := range childWorkflows {
		jobID, parsedChild := sw.Job()
		if parsedChild == nil {
			continue
		}
		needs := parsedChild.Needs()
		if err := sw.SetJob(jobID, parsedChild.EraseNeeds()); err != nil {
			return err
		}
		payload, err := sw.Marshal()
		if err != nil {
			return fmt.Errorf("marshal child %q under caller %d: %w", jobID, caller.ID, err)
		}

		parsedChild.Name = util.EllipsisDisplayString(parsedChild.Name, 255)

		// AttemptJobID: prefer a prior-attempt match by (JobID, Name) and fall back to a fresh allocator value for newly-appearing logical jobs.
		// The two-level key disambiguates matrix instances (same JobID, different Names) and distinct jobs that legally share the same Name (different JobIDs).
		var attemptJobID int64
		if priorChild, ok := priorChildren[jobID][parsedChild.Name]; ok {
			attemptJobID = priorChild.AttemptJobID
		} else {
			attemptJobID, err = actions_model.GetNextAttemptJobID(ctx, run.ID)
			if err != nil {
				return fmt.Errorf("alloc attempt_job_id for child %q: %w", jobID, err)
			}
		}
		child := &actions_model.ActionRunJob{
			RunID:                   run.ID,
			RunAttemptID:            attempt.ID,
			RepoID:                  run.RepoID,
			OwnerID:                 run.OwnerID,
			CommitSHA:               run.CommitSHA,
			IsForkPullRequest:       run.IsForkPullRequest,
			Name:                    parsedChild.Name,
			Attempt:                 attempt.Attempt,
			WorkflowPayload:         payload,
			JobID:                   jobID,
			AttemptJobID:            attemptJobID,
			Needs:                   needs,
			RunsOn:                  parsedChild.RunsOn(),
			ContinueOnError:         parsedChild.GetContinueOnError(),
			Status:                  actions_model.StatusBlocked,
			ParentJobID:             caller.ID,
			WorkflowSourceRepoID:    sourceRepoID,
			WorkflowSourceCommitSHA: sourceCommitSHA,
		}
		if perms := ExtractJobPermissionsFromWorkflow(sw, parsedChild); perms != nil {
			child.TokenPermissions = perms
		}
		if parsedChild.Uses != "" {
			child.IsReusableCaller = true
			child.CallUses = parsedChild.Uses
		}
		if err := db.Insert(ctx, child); err != nil {
			return fmt.Errorf("insert child %q under caller %d: %w", jobID, caller.ID, err)
		}
	}
	return nil
}

// ResolveUses normalizes and parses a reusable workflow `uses:` value.
// It first rewrites an absolute URL pointing to this instance into the cross-repo form (rejecting external URLs),
// then validates the syntax via jobparser.ParseUses.
func ResolveUses(ctx context.Context, uses string) (*jobparser.UsesRef, error) {
	// Rewrite a local-instance URL to the equivalent cross-repo form "owner/repo/.gitea/workflows/file.yml@ref".
	if strings.HasPrefix(uses, "http://") || strings.HasPrefix(uses, "https://") {
		// ParseGiteaSiteURL returns nil for URLs that do not belong to this instance.
		gsu := httplib.ParseGiteaSiteURL(ctx, uses)
		if gsu == nil {
			return nil, fmt.Errorf("unsupported reusable workflow URL %q: an absolute URL must point to this Gitea instance (%s)", uses, setting.AppURL)
		}
		// RoutePath is the instance-relative path (AppSubURL already stripped), e.g. "/owner/repo/.gitea/workflows/file.yml@ref".
		uses = strings.TrimPrefix(gsu.RoutePath, "/")
	}
	return jobparser.ParseUses(uses)
}

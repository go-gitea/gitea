// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/log"

	"go.yaml.in/yaml/v4"
	"xorm.io/builder"
)

// markMatrixAsEvaluatedAndSkip marks a job as evaluated and skipped unconditionally.
// Used when matrix cannot be expanded or dependency checks fail (no concurrent
// insertion has taken place, so no race is possible).
func markMatrixAsEvaluatedAndSkip(ctx context.Context, job *actions_model.ActionRunJob, reason string) error {
	job.IsMatrixEvaluated = true
	job.Status = actions_model.StatusSkipped
	if _, err := actions_model.UpdateRunJob(ctx, job, nil, "is_matrix_evaluated", "status"); err != nil {
		log.Error("Failed to mark job %d (JobID: %s) as evaluated and skipped: %v", job.ID, job.JobID, err)
		return err
	}
	log.Debug("Marked job %d (JobID: %s) as evaluated and skipped: %s", job.ID, job.JobID, reason)
	return nil
}

// checkTaskNeedsReady verifies if all task dependencies are completed.
// Returns (taskNeeds, allDone, error)
func checkTaskNeedsReady(ctx context.Context, job *actions_model.ActionRunJob) (map[string]*TaskNeed, bool, error) {
	taskNeeds, err := FindTaskNeeds(ctx, job)
	if err != nil {
		return nil, false, fmt.Errorf("find task needs: %w", err)
	}

	log.Debug("Found %d task needs for job %d (JobID: %s)", len(taskNeeds), job.ID, job.JobID)

	var pendingNeeds []string
	for jobID, taskNeed := range taskNeeds {
		if !taskNeed.Result.IsDone() {
			pendingNeeds = append(pendingNeeds, fmt.Sprintf("%s(%s)", jobID, taskNeed.Result))
		}
	}

	if len(pendingNeeds) > 0 {
		log.Debug("Matrix re-evaluation deferred for job %d: pending needs: %v", job.ID, pendingNeeds)
		return taskNeeds, false, nil
	}

	return taskNeeds, true, nil
}

// ExtractRawStrategies extracts strategy definitions from the raw workflow content.
// Returns a map of jobID to strategy YAML for jobs that have matrix dependencies.
func ExtractRawStrategies(content []byte) (map[string]string, error) {
	var workflowDef struct {
		Jobs map[string]struct {
			Strategy any `yaml:"strategy"`
			Needs    any `yaml:"needs"`
		} `yaml:"jobs"`
	}

	if err := yaml.Unmarshal(content, &workflowDef); err != nil {
		return nil, err
	}

	strategies := make(map[string]string)
	for jobID, jobDef := range workflowDef.Jobs {
		if jobDef.Strategy == nil {
			continue
		}

		var needsList []string
		switch needs := jobDef.Needs.(type) {
		case string:
			needsList = append(needsList, needs)
		case []any:
			for _, need := range needs {
				if needStr, ok := need.(string); ok {
					needsList = append(needsList, needStr)
				}
			}
		}

		if len(needsList) > 0 {
			if strategyBytes, err := yaml.Marshal(jobDef.Strategy); err == nil {
				strategies[jobID] = string(strategyBytes)
			}
		}
	}

	return strategies, nil
}

// HasMatrixWithNeeds reports whether rawStrategy contains a matrix value whose
// expression tree references needs.<id>.outputs.<key>.
// It walks the parsed YAML tree to avoid false positives from values such as
// "os: [needs.review-runner]" that merely contain the substring "needs.".
func HasMatrixWithNeeds(rawStrategy string) bool {
	if rawStrategy == "" {
		return false
	}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(rawStrategy), &root); err != nil {
		return false
	}

	// The top-level document node wraps a single mapping node.
	doc := &root
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		doc = doc.Content[0]
	}

	// Find the "matrix" key inside the strategy mapping.
	var matrixNode *yaml.Node
	if doc.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(doc.Content); i += 2 {
			if doc.Content[i].Value == "matrix" {
				matrixNode = doc.Content[i+1]
				break
			}
		}
	}
	if matrixNode == nil {
		return false
	}

	return yamlNodeContainsNeedsOutputsExpr(matrixNode)
}

// yamlNodeContainsNeedsOutputsExpr recursively inspects a yaml.Node and
// returns true if any scalar value contains a GitHub Actions expression of
// the form ${{ ... needs.<id>.outputs.<key> ... }}.
func yamlNodeContainsNeedsOutputsExpr(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.ScalarNode {
		return containsNeedsOutputsExpr(node.Value)
	}
	return slices.ContainsFunc(node.Content, yamlNodeContainsNeedsOutputsExpr)
}

// containsNeedsOutputsExpr returns true when s contains an Actions expression
// (${{ ... }}) that references needs.<id>.outputs.<key>.
// A bare "needs." substring outside an expression block is not a match.
func containsNeedsOutputsExpr(s string) bool {
	if !strings.Contains(s, "${{") {
		return false
	}
	for i := 0; i < len(s); {
		start := strings.Index(s[i:], "${{")
		if start == -1 {
			break
		}
		start += i
		end := strings.Index(s[start:], "}}")
		if end == -1 {
			break
		}
		end += start
		expr := s[start : end+2]
		if strings.Contains(expr, "needs.") && strings.Contains(expr, ".outputs.") {
			return true
		}
		i = end + 2
	}
	return false
}

// ReEvaluateMatrixForJobWithNeeds re-evaluates the matrix strategy of a job once all its
// dependent jobs are done. It expands the matrix using job outputs and inserts the resulting
// ActionRunJobs. The original placeholder job is marked as evaluated and skipped.
// Returns nil, nil if the job is not ready yet or has nothing to do.
func ReEvaluateMatrixForJobWithNeeds(ctx context.Context, job *actions_model.ActionRunJob, vars map[string]string) ([]*actions_model.ActionRunJob, error) {
	if job.IsMatrixEvaluated || job.RawStrategy == "" {
		return nil, nil
	}

	log.Debug("Starting matrix re-evaluation for job %d (JobID: %s)", job.ID, job.JobID)

	// skipWithError marks the job as evaluated+skipped and wraps any secondary error.
	skipWithError := func(reason string, origErr error) ([]*actions_model.ActionRunJob, error) {
		if markErr := markMatrixAsEvaluatedAndSkip(ctx, job, reason); markErr != nil {
			return nil, fmt.Errorf("%w; additionally failed to mark as evaluated: %v", origErr, markErr)
		}
		return nil, origErr
	}

	taskNeeds, allDone, err := checkTaskNeedsReady(ctx, job)
	if err != nil {
		log.Error("Matrix re-evaluation error for job %d: check task needs: %v", job.ID, err)
		return skipWithError(fmt.Sprintf("task needs check failed: %v", err), fmt.Errorf("check task needs: %w", err))
	}
	if !allDone {
		return nil, nil
	}

	if job.Run == nil {
		if err := job.LoadRun(ctx); err != nil {
			return nil, fmt.Errorf("load run: %w", err)
		}
	}
	if job.Run == nil {
		return nil, errors.New("run not found after loading")
	}
	if err := job.Run.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("load run attributes: %w", err)
	}

	giteaCtx := GenerateGiteaContext(ctx, job.Run, nil, job)

	jobOutputs := make(map[string]map[string]string, len(taskNeeds))
	jobResults := make(map[string]string, len(taskNeeds))
	for jobID, need := range taskNeeds {
		jobOutputs[jobID] = need.Outputs
		jobResults[jobID] = need.Result.String()
	}

	workflowYAML, err := constructWorkflowWithNeeds(job, taskNeeds)
	if err != nil {
		return skipWithError(fmt.Sprintf("workflow construction failed: %v", err), fmt.Errorf("construct workflow: %w", err))
	}

	parsedJobs, err := jobparser.Parse(
		workflowYAML,
		jobparser.WithVars(vars),
		jobparser.WithGitContext(giteaCtx.ToGitHubContext()),
		jobparser.WithJobOutputs(jobOutputs),
		jobparser.WithJobResults(jobResults),
	)
	if err != nil {
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, fmt.Sprintf("parse failed: %v", err))
	}

	// One parsed workflow per combination; needs are kept on the model and erased from the
	// payload, as at initial planning time.
	type matrixCombo struct {
		name    string
		payload []byte
		runsOn  []string
		needs   []string
	}
	var combos []matrixCombo
	for _, sw := range parsedJobs {
		id, jobDef := sw.Job()
		if jobDef == nil || id != job.JobID {
			continue
		}
		combo := matrixCombo{name: jobDef.Name, runsOn: jobDef.RunsOn(), needs: jobDef.Needs()}
		if err := sw.SetJob(id, jobDef.EraseNeeds()); err != nil {
			return nil, fmt.Errorf("erase needs for job %s: %w", id, err)
		}
		if combo.payload, err = sw.Marshal(); err != nil {
			return nil, fmt.Errorf("marshal expanded job %s: %w", id, err)
		}
		combos = append(combos, combo)
	}

	if len(combos) == 0 {
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, "matrix expanded to no combinations")
	}

	// Reuse the placeholder as the first combination and insert the rest as siblings: no phantom
	// skipped job is left to poison downstream needs, and siblings inherit attempt + permissions.
	var children []*actions_model.ActionRunJob
	if err := db.WithTx(ctx, func(txCtx context.Context) error {
		maxAttemptJobID, err := actions_model.GetMaxAttemptJobID(txCtx, job.RunID, job.RunAttemptID)
		if err != nil {
			return err
		}
		for i := 1; i < len(combos); i++ {
			children = append(children, &actions_model.ActionRunJob{
				RunID:             job.RunID,
				RunAttemptID:      job.RunAttemptID,
				RepoID:            job.RepoID,
				OwnerID:           job.OwnerID,
				CommitSHA:         job.CommitSHA,
				IsForkPullRequest: job.IsForkPullRequest,
				Name:              combos[i].name,
				Attempt:           job.Attempt,
				WorkflowPayload:   combos[i].payload,
				JobID:             job.JobID,
				AttemptJobID:      maxAttemptJobID + int64(i),
				Needs:             combos[i].needs,
				RunsOn:            combos[i].runsOn,
				TokenPermissions:  job.TokenPermissions,
				Status:            actions_model.StatusWaiting,
			})
		}

		// Atomic claim: only one concurrent caller flips the placeholder out of Blocked.
		job.Name = combos[0].name
		job.WorkflowPayload = combos[0].payload
		job.RunsOn = combos[0].runsOn
		job.IsMatrixEvaluated = true
		job.Status = actions_model.StatusWaiting
		affected, err := actions_model.UpdateRunJob(txCtx, job,
			builder.Eq{"is_matrix_evaluated": false, "status": actions_model.StatusBlocked},
			"name", "workflow_payload", "runs_on", "is_matrix_evaluated", "status")
		if err != nil {
			return err
		}
		if affected != 1 {
			children = nil
			return nil
		}
		return actions_model.InsertActionRunJobs(txCtx, children)
	}); err != nil {
		return nil, fmt.Errorf("expand matrix for job %d: %w", job.ID, err)
	}

	return children, nil
}

// constructWorkflowWithNeeds creates a workflow YAML that includes the target job
// and stub definitions for its dependencies so the jobparser can resolve needs.*.outputs expressions.
func constructWorkflowWithNeeds(job *actions_model.ActionRunJob, taskNeeds map[string]*TaskNeed) ([]byte, error) {
	var jobWorkflow map[string]any
	if err := yaml.Unmarshal(job.WorkflowPayload, &jobWorkflow); err != nil {
		return nil, fmt.Errorf("unmarshal job workflow: %w", err)
	}

	jobsSection, ok := jobWorkflow["jobs"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid jobs section in workflow")
	}

	newJobs := make(map[string]any)

	for needJobID, taskNeed := range taskNeeds {
		stubJob := map[string]any{
			"runs-on": "ubuntu-latest",
			"outputs": taskNeed.Outputs,
			"steps":   []any{},
		}
		newJobs[needJobID] = stubJob
	}

	maps.Copy(newJobs, jobsSection)

	// The WorkflowPayload may contain a normalised/wrapped matrix (e.g.
	// version: ["${{ fromJson(...) }}"]). Restore the original scalar expression
	// from RawStrategy so jobparser.Parse() can expand it correctly with job outputs.
	// Also drop the pre-baked "name" so jobparser regenerates it per matrix combination
	// (e.g. "build (1)", "build (2)", …) instead of "build (Array)".
	// Critically, re-add "needs" because EraseNeeds() removed them from WorkflowPayload:
	// without needs, NewInterpeter builds an empty Needs context and
	// "needs.generate.outputs.*" expressions can never be evaluated.
	if targetJobDef, ok := newJobs[job.JobID]; ok {
		if targetJobMap, ok := targetJobDef.(map[string]any); ok {
			delete(targetJobMap, "name")
			needsKeys := make([]string, 0, len(taskNeeds))
			for needJobID := range taskNeeds {
				needsKeys = append(needsKeys, needJobID)
			}
			sort.Strings(needsKeys)
			targetJobMap["needs"] = needsKeys
			if job.RawStrategy != "" {
				var rawStrategyMap map[string]any
				if err := yaml.Unmarshal([]byte(job.RawStrategy), &rawStrategyMap); err == nil {
					targetJobMap["strategy"] = rawStrategyMap
				}
			}
			newJobs[job.JobID] = targetJobMap
		}
	}

	workflow := map[string]any{
		"name": "matrix-expansion",
		"on":   "push",
		"jobs": newJobs,
	}

	return yaml.Marshal(workflow)
}

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/log"

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

// claimMatrixExpansion atomically marks the placeholder job as evaluated+skipped
// only when it is still in its original state (is_matrix_evaluated=false AND
// status=Blocked). Returns (true, nil) when this caller wins the claim, or
// (false, nil) when another concurrent process already claimed it.
// This is the concurrency guard that prevents double-expansion.
func claimMatrixExpansion(ctx context.Context, job *actions_model.ActionRunJob) (bool, error) {
	job.IsMatrixEvaluated = true
	job.Status = actions_model.StatusSkipped
	n, err := actions_model.UpdateRunJob(ctx, job,
		builder.Eq{"is_matrix_evaluated": false, "status": actions_model.StatusBlocked},
		"is_matrix_evaluated", "status")
	if err != nil {
		return false, err
	}
	return n == 1, nil
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
	for _, child := range node.Content {
		if yamlNodeContainsNeedsOutputsExpr(child) {
			return true
		}
	}
	return false
}

// containsNeedsOutputsExpr returns true when s contains a GitHub Actions
// expression (${{ ... }}) that references needs.<id>.outputs.<key>.
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

	if !HasMatrixWithNeeds(job.RawStrategy) {
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, "no needs-dependent matrix found")
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

	mergedVars := mergeNeedsIntoVars(vars, taskNeeds)

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
		log.Error("Matrix re-evaluation error for job %d: construct workflow: %v", job.ID, err)
		return skipWithError(fmt.Sprintf("workflow construction failed: %v", err), fmt.Errorf("construct workflow: %w", err))
	}

	parsedJobs, err := jobparser.Parse(
		workflowYAML,
		jobparser.WithVars(mergedVars),
		jobparser.WithGitContext(giteaCtx.ToGitHubContext()),
		jobparser.WithJobOutputs(jobOutputs),
		jobparser.WithJobResults(jobResults),
	)
	if err != nil {
		log.Error("Matrix parse error for job %d (RawStrategy: %s): %v", job.ID, job.RawStrategy, err)
		if markErr := markMatrixAsEvaluatedAndSkip(ctx, job, fmt.Sprintf("parse failed: %v", err)); markErr != nil {
			return nil, fmt.Errorf("parse workflow: %w; additionally failed to mark as evaluated: %v", err, markErr)
		}
		return nil, nil
	}

	if len(parsedJobs) == 0 {
		log.Debug("No jobs generated from matrix expansion for job %d (JobID: %s)", job.ID, job.JobID)
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, "no jobs generated from matrix expansion")
	}

	log.Debug("Parsed %d matrix combinations for job %d (JobID: %s)", len(parsedJobs), job.ID, job.JobID)

	var newJobs []*actions_model.ActionRunJob
	for i, sw := range parsedJobs {
		id, jobDef := sw.Job()
		if jobDef == nil {
			log.Warn("Skipped nil jobDef at index %d for job %d (JobID: %s)", i, job.ID, job.JobID)
			continue
		}
		if id != job.JobID {
			continue
		}
		needs := jobDef.Needs()
		if err := sw.SetJob(id, jobDef.EraseNeeds()); err != nil {
			log.Error("Failed to erase needs from job %s (matrix expansion for job %d): %v", id, job.ID, err)
			continue
		}
		payload, _ := sw.Marshal()
		newJobs = append(newJobs, &actions_model.ActionRunJob{
			RunID:             job.RunID,
			RepoID:            job.RepoID,
			OwnerID:           job.OwnerID,
			CommitSHA:         job.CommitSHA,
			IsForkPullRequest: job.IsForkPullRequest,
			Name:              jobDef.Name,
			WorkflowPayload:   payload,
			JobID:             id,
			Needs:             needs,
			RunsOn:            jobDef.RunsOn(),
			Status:            actions_model.StatusWaiting,
		})
	}

	if len(newJobs) == 0 {
		log.Warn("No valid jobs created from matrix expansion for job %d (JobID: %s), total parsed: %d", job.ID, job.JobID, len(parsedJobs))
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, fmt.Sprintf("no valid jobs created (parsed: %d)", len(parsedJobs)))
	}

	// Atomically claim the placeholder and insert the new jobs in one transaction.
	// The conditional WHERE in claimMatrixExpansion (is_matrix_evaluated=false AND
	// status=Blocked) ensures only one concurrent caller can win. The second caller
	// sees n==0 and rolls back without inserting duplicate jobs.
	var claimed bool
	if err := db.WithTx(ctx, func(txCtx context.Context) error {
		var txErr error
		claimed, txErr = claimMatrixExpansion(txCtx, job)
		if txErr != nil {
			return txErr
		}
		if !claimed {
			return nil
		}
		return actions_model.InsertActionRunJobs(txCtx, newJobs)
	}); err != nil {
		log.Error("Matrix expansion transaction failed for job %d (JobID: %s): %v", job.ID, job.JobID, err)
		return nil, fmt.Errorf("matrix expansion transaction: %w", err)
	}
	if !claimed {
		log.Warn("Matrix placeholder job %d (JobID: %s) was already claimed by a concurrent process; skipping",
			job.ID, job.JobID)
		return nil, nil
	}

	log.Debug("Matrix re-evaluation complete for job %d (JobID: %s): created %d new jobs",
		job.ID, job.JobID, len(newJobs))

	return newJobs, nil
}

// mergeNeedsIntoVars converts task needs outputs into variables for expression evaluation.
func mergeNeedsIntoVars(baseVars map[string]string, taskNeeds map[string]*TaskNeed) map[string]string {
	merged := make(map[string]string)
	maps.Copy(merged, baseVars)
	for jobID, taskNeed := range taskNeeds {
		for outputKey, outputValue := range taskNeed.Outputs {
			key := fmt.Sprintf("needs.%s.outputs.%s", jobID, outputKey)
			merged[key] = outputValue
		}
	}
	return merged
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

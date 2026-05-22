// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/actions/jobparser"
	"code.gitea.io/gitea/modules/log"

	"go.yaml.in/yaml/v4"
)

// markMatrixAsEvaluatedAndSkip marks a job's matrix as evaluated and skipped.
// Used when matrix cannot be expanded or dependency checks fail.
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

	// Check if any task needs are not done
	var pendingNeeds []string
	for jobID, taskNeed := range taskNeeds {
		if !taskNeed.Result.IsDone() {
			pendingNeeds = append(pendingNeeds, fmt.Sprintf("%s(%s)", jobID, taskNeed.Result))
		}
	}

	if len(pendingNeeds) > 0 {
		log.Debug("Matrix re-evaluation deferred for job %d: pending needs: %v", job.ID, pendingNeeds)
		GetMatrixMetrics().RecordDeferred()
		return taskNeeds, false, nil
	}

	return taskNeeds, true, nil
}

// ExtractRawStrategies extracts strategy definitions from the raw workflow content
// Returns a map of jobID to strategy YAML for jobs that have matrix dependencies
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

		// Check if this job has needs (dependencies)
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

		// Only store strategy for jobs with dependencies
		if len(needsList) > 0 {
			if strategyBytes, err := yaml.Marshal(jobDef.Strategy); err == nil {
				strategies[jobID] = string(strategyBytes)
			}
		}
	}

	return strategies, nil
}

// HasMatrixWithNeeds checks if a job's strategy contains a matrix that depends on job outputs
func HasMatrixWithNeeds(rawStrategy string) bool {
	if rawStrategy == "" {
		return false
	}

	var strategy map[string]any
	if err := yaml.Unmarshal([]byte(rawStrategy), &strategy); err != nil {
		return false
	}

	matrix, ok := strategy["matrix"]
	if !ok {
		return false
	}

	// Check if any matrix value contains "needs." reference
	matrixStr := fmt.Sprintf("%v", matrix)
	return strings.Contains(matrixStr, "needs.")
}

// ReEvaluateMatrixForJobWithNeeds re-evaluates the matrix strategy of a job once all its
// dependent jobs are done. It expands the matrix using job outputs and inserts the resulting
// ActionRunJobs. The original placeholder job is marked as evaluated and skipped.
// Returns nil, nil if the job is not ready yet or has nothing to do.
func ReEvaluateMatrixForJobWithNeeds(ctx context.Context, job *actions_model.ActionRunJob, vars map[string]string) ([]*actions_model.ActionRunJob, error) {
	startTime := time.Now()

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

	// Check if dependencies are ready BEFORE doing expensive parsing
	taskNeeds, allDone, err := checkTaskNeedsReady(ctx, job)
	if err != nil {
		log.Error("Matrix re-evaluation error for job %d: check task needs: %v", job.ID, err)
		return skipWithError(fmt.Sprintf("task needs check failed: %v", err), fmt.Errorf("check task needs: %w", err))
	}
	if !allDone {
		return nil, nil // wait for dependencies
	}

	mergedVars := mergeNeedsIntoVars(vars, taskNeeds)

	// Load run and its attributes for expression evaluation context
	if job.Run == nil {
		if err := job.LoadRun(ctx); err != nil {
			GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
			return nil, fmt.Errorf("load run: %w", err)
		}
	}
	if job.Run == nil {
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, errors.New("run not found after loading")
	}
	if err := job.Run.LoadAttributes(ctx); err != nil {
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, fmt.Errorf("load run attributes: %w", err)
	}

	giteaCtx := GenerateGiteaContext(ctx, job.Run, nil, job)

	jobOutputs := make(map[string]map[string]string, len(taskNeeds))
	jobResults := make(map[string]string, len(taskNeeds))
	for jobID, need := range taskNeeds {
		jobOutputs[jobID] = need.Outputs
		jobResults[jobID] = need.Result.String()
	}

	// Build a minimal workflow containing this job and stubs for its dependencies,
	// so the jobparser can resolve needs.*.outputs.* expressions.
	workflowYAML, err := constructWorkflowWithNeeds(job, taskNeeds)
	if err != nil {
		log.Error("Matrix re-evaluation error for job %d: construct workflow: %v", job.ID, err)
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return skipWithError(fmt.Sprintf("workflow construction failed: %v", err), fmt.Errorf("construct workflow: %w", err))
	}

	// Track cache hit/miss for metrics (actual caching of parsed objects is future work)
	cacheKey := computeCacheKey(workflowYAML, taskNeeds)
	if cache := getWorkflowParseCache(); cache != nil {
		if _, hit := cache.Get(cacheKey); hit {
			log.Debug("Cache hit for workflow parse (job %d, key: %.16s)", job.ID, cacheKey)
			GetMatrixMetrics().RecordCacheHit()
		} else {
			GetMatrixMetrics().RecordCacheMiss()
		}
	}

	parseStart := time.Now()
	parsedJobs, err := jobparser.Parse(
		workflowYAML,
		jobparser.WithVars(mergedVars),
		jobparser.WithGitContext(giteaCtx.ToGitHubContext()),
		jobparser.WithJobOutputs(jobOutputs),
		jobparser.WithJobResults(jobResults),
	)
	GetMatrixMetrics().RecordParseTime(time.Since(parseStart))

	if err != nil {
		log.Error("Matrix parse error for job %d (RawStrategy: %s): %v", job.ID, job.RawStrategy, err)
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		// Don't propagate parse errors — mark as evaluated to avoid retry loops
		if markErr := markMatrixAsEvaluatedAndSkip(ctx, job, fmt.Sprintf("parse failed: %v", err)); markErr != nil {
			return nil, fmt.Errorf("parse workflow: %w; additionally failed to mark as evaluated: %v", err, markErr)
		}
		return nil, nil
	}

	if len(parsedJobs) == 0 {
		log.Debug("No jobs generated from matrix expansion for job %d (JobID: %s)", job.ID, job.JobID)
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, "no jobs generated from matrix expansion")
	}

	// Cache successful parse result (YAML payload for metrics tracking)
	if cache := getWorkflowParseCache(); cache != nil && len(parsedJobs) > 0 {
		cache.Set(cacheKey, workflowYAML)
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
			// Skip dependency stubs — we only want matrix-expanded entries for the target job
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
			// All dependency jobs are already done at this point; start as Waiting.
			Status: actions_model.StatusWaiting,
		})
	}

	if len(newJobs) == 0 {
		log.Warn("No valid jobs created from matrix expansion for job %d (JobID: %s), total parsed: %d", job.ID, job.JobID, len(parsedJobs))
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, markMatrixAsEvaluatedAndSkip(ctx, job, fmt.Sprintf("no valid jobs created (parsed: %d)", len(parsedJobs)))
	}

	insertStart := time.Now()
	if err := actions_model.InsertActionRunJobs(ctx, newJobs); err != nil {
		log.Error("Matrix insertion error: failed to insert %d new matrix jobs for job %d (JobID: %s): %v", len(newJobs), job.ID, job.JobID, err)
		GetMatrixMetrics().RecordInsertTime(time.Since(insertStart))
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, fmt.Errorf("insert new jobs: %w", err)
	}
	GetMatrixMetrics().RecordInsertTime(time.Since(insertStart))

	// Mark the placeholder job as evaluated and skipped so it is never run
	if err := markMatrixAsEvaluatedAndSkip(ctx, job, fmt.Sprintf("successfully created %d new jobs", len(newJobs))); err != nil {
		// Non-fatal: new jobs are already inserted
		log.Error("Failed to mark placeholder job %d as evaluated after creating %d new jobs: %v", job.ID, len(newJobs), err)
	}

	totalTime := time.Since(startTime)
	GetMatrixMetrics().RecordReevaluation(totalTime, true, int64(len(newJobs)))
	log.Info("Matrix re-evaluation complete for job %d (JobID: %s): %d new jobs in %dms",
		job.ID, job.JobID, len(newJobs), totalTime.Milliseconds())

	return newJobs, nil
}

// mergeNeedsIntoVars converts task needs outputs into variables for expression evaluation
func mergeNeedsIntoVars(baseVars map[string]string, taskNeeds map[string]*TaskNeed) map[string]string {
	merged := make(map[string]string)

	// Copy base vars
	maps.Copy(merged, baseVars)

	// Add needs outputs as variables in format: needs.<job_id>.outputs.<output_name>
	for jobID, taskNeed := range taskNeeds {
		for outputKey, outputValue := range taskNeed.Outputs {
			key := fmt.Sprintf("needs.%s.outputs.%s", jobID, outputKey)
			merged[key] = outputValue
		}
	}

	return merged
}

// constructWorkflowWithNeeds creates a workflow YAML that includes the target job
// and stub definitions for its dependencies so the jobparser can resolve needs.*.outputs expressions
func constructWorkflowWithNeeds(job *actions_model.ActionRunJob, taskNeeds map[string]*TaskNeed) ([]byte, error) {
	// Parse the original job's workflow payload to get the job definition
	var jobWorkflow map[string]any
	if err := yaml.Unmarshal(job.WorkflowPayload, &jobWorkflow); err != nil {
		return nil, fmt.Errorf("unmarshal job workflow: %w", err)
	}

	// Extract the job definition from the parsed workflow
	jobsSection, ok := jobWorkflow["jobs"].(map[string]any)
	if !ok {
		return nil, errors.New("invalid jobs section in workflow")
	}

	// Create a new workflow with the target job and stub jobs for dependencies
	newJobs := make(map[string]any)

	// Add stub jobs for each dependency with their outputs
	for needJobID, taskNeed := range taskNeeds {
		stubJob := map[string]any{
			"runs-on": "ubuntu-latest",
			"outputs": taskNeed.Outputs,
			"steps":   []any{},
		}
		newJobs[needJobID] = stubJob
	}

	// Add the actual job we want to expand (with matrix and needs)
	maps.Copy(newJobs, jobsSection)

	// Construct the full workflow
	workflow := map[string]any{
		"name": "matrix-expansion",
		"on":   "push",
		"jobs": newJobs,
	}

	return yaml.Marshal(workflow)
}

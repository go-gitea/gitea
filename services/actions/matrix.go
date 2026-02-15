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
	"code.gitea.io/gitea/modules/log"

	"github.com/nektos/act/pkg/jobparser"
	"go.yaml.in/yaml/v4"
)

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

// hasMatrixWithNeeds checks if a job's strategy contains a matrix that depends on job outputs
func hasMatrixWithNeeds(rawStrategy string) bool {
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

// ReEvaluateMatrixForJobWithNeeds re-evaluates the matrix strategy of a job using outputs from dependent jobs
// If the matrix depends on job outputs and all dependent jobs are done, it will:
// 1. Evaluate the matrix with the job outputs
// 2. Create new ActionRunJobs for each matrix combination
// 3. Return the newly created jobs
func ReEvaluateMatrixForJobWithNeeds(ctx context.Context, job *actions_model.ActionRunJob, vars map[string]string) ([]*actions_model.ActionRunJob, error) {
	startTime := time.Now()

	if job.IsMatrixEvaluated || job.RawStrategy == "" {
		return nil, nil
	}

	if !hasMatrixWithNeeds(job.RawStrategy) {
		// Mark as evaluated since there's no needs-dependent matrix
		job.IsMatrixEvaluated = true
		log.Debug("Matrix re-evaluation skipped for job %d: no needs-dependent matrix found", job.ID)
		return nil, nil
	}

	log.Debug("Starting matrix re-evaluation for job %d (JobID: %s)", job.ID, job.JobID)

	// Get the outputs from dependent jobs
	taskNeeds, err := FindTaskNeeds(ctx, job)
	if err != nil {
		errMsg := fmt.Sprintf("failed to find task needs for job %d (JobID: %s): %v", job.ID, job.JobID, err)
		log.Error("Matrix re-evaluation error: %s", errMsg)
		return nil, fmt.Errorf("find task needs: %w", err)
	}

	log.Debug("Found %d task needs for job %d (JobID: %s)", len(taskNeeds), job.ID, job.JobID)

	// If any task needs are not done, we can't evaluate yet
	pendingNeeds := []string{}
	for jobID, taskNeed := range taskNeeds {
		if !taskNeed.Result.IsDone() {
			pendingNeeds = append(pendingNeeds, fmt.Sprintf("%s(%s)", jobID, taskNeed.Result))
		}
	}
	if len(pendingNeeds) > 0 {
		log.Debug("Matrix re-evaluation deferred for job %d: pending needs: %v", job.ID, pendingNeeds)
		GetMatrixMetrics().RecordDeferred()
		return nil, nil
	}

	// Merge vars with needs outputs
	mergedVars := mergeNeedsIntoVars(vars, taskNeeds)
	log.Debug("Merged %d variables with needs outputs for job %d", len(mergedVars), job.ID)

	// Load the original run to get workflow context
	if job.Run == nil {
		if err := job.LoadRun(ctx); err != nil {
			errMsg := fmt.Sprintf("failed to load run for job %d (JobID: %s): %v", job.ID, job.JobID, err)
			log.Error("Matrix re-evaluation error: %s", errMsg)
			GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
			return nil, fmt.Errorf("load run: %w", err)
		}
	}

	// Verify run is not nil after loading
	if job.Run == nil {
		errMsg := fmt.Sprintf("run is nil for job %d (JobID: %s) after loading", job.ID, job.JobID)
		log.Error("Matrix re-evaluation error: %s", errMsg)
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, errors.New("run not found: nil run")
	}

	// Load run attributes (TriggerUser, Repo, etc.)
	if err := job.Run.LoadAttributes(ctx); err != nil {
		errMsg := fmt.Sprintf("failed to load run attributes for job %d (JobID: %s): %v", job.ID, job.JobID, err)
		log.Error("Matrix re-evaluation error: %s", errMsg)
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, fmt.Errorf("load run attributes: %w", err)
	}

	// Create the giteaCtx for expression evaluation
	giteaCtx := GenerateGiteaContext(job.Run, job)

	// Convert taskNeeds to job outputs format for jobparser
	jobOutputs := make(map[string]map[string]string)
	jobResults := make(map[string]string)
	for jobID, taskNeed := range taskNeeds {
		jobOutputs[jobID] = taskNeed.Outputs
		jobResults[jobID] = taskNeed.Result.String()
	}

	// We need to construct a workflow that includes both this job AND its dependencies
	// so that the jobparser can resolve needs.*.outputs.* expressions
	workflowYAML, err := constructWorkflowWithNeeds(job, taskNeeds)
	if err != nil {
		log.Error("Failed to construct workflow for job %d (JobID: %s): %v", job.ID, job.JobID, err)
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, nil
	}

	// Parse the constructed workflow with job outputs to expand the matrix
	parseStartTime := time.Now()
	jobs, err := jobparser.Parse(
		workflowYAML,
		jobparser.WithVars(mergedVars),
		jobparser.WithGitContext(giteaCtx.ToGitHubContext()),
		jobparser.WithJobOutputs(jobOutputs),
		jobparser.WithJobResults(jobResults),
	)
	parseTime := time.Since(parseStartTime)
	GetMatrixMetrics().RecordParseTime(parseTime)

	if err != nil {
		// If parsing fails, we can't expand the matrix
		// Mark as evaluated and skip
		job.IsMatrixEvaluated = true
		errMsg := fmt.Sprintf("failed to parse workflow payload for job %d (JobID: %s) during matrix expansion. Error: %v. RawStrategy: %s",
			job.ID, job.JobID, err, job.RawStrategy)
		log.Error("Matrix parse error: %s", errMsg)
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, nil
	}

	if len(jobs) == 0 {
		job.IsMatrixEvaluated = true
		log.Debug("No jobs generated from matrix expansion for job %d (JobID: %s)", job.ID, job.JobID)
		return nil, nil
	}

	log.Debug("Parsed %d matrix combinations for job %d (JobID: %s)", len(jobs), job.ID, job.JobID)

	// Create new ActionRunJobs for each parsed workflow (each matrix combination)
	newJobs := make([]*actions_model.ActionRunJob, 0)

	for i, parsedSingleWorkflow := range jobs {
		id, jobDef := parsedSingleWorkflow.Job()
		if jobDef == nil {
			log.Warn("Skipped nil jobDef at index %d for job %d (JobID: %s)", i, job.ID, job.JobID)
			continue
		}

		// Skip the original job ID - we only want the matrix-expanded versions
		if id == job.JobID {
			log.Debug("Skipped original job ID %s in matrix expansion for job %d", id, job.ID)
			continue
		}

		// Erase needs from the payload before storing
		needs := jobDef.Needs()
		if err := parsedSingleWorkflow.SetJob(id, jobDef.EraseNeeds()); err != nil {
			log.Error("Failed to erase needs from job %s (matrix expansion for job %d): %v", id, job.ID, err)
			continue
		}

		payload, _ := parsedSingleWorkflow.Marshal()

		newJob := &actions_model.ActionRunJob{
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
			Status:            actions_model.StatusBlocked,
		}

		newJobs = append(newJobs, newJob)
	}

	// If no new jobs were created, mark as evaluated
	if len(newJobs) == 0 {
		job.IsMatrixEvaluated = true
		log.Warn("No valid jobs created from matrix expansion for job %d (JobID: %s). Original jobs: %d", job.ID, job.JobID, len(jobs))
		return nil, nil
	}

	// Insert the new jobs into database
	insertStartTime := time.Now()
	if err := actions_model.InsertActionRunJobs(ctx, newJobs); err != nil {
		insertTime := time.Since(insertStartTime)
		GetMatrixMetrics().RecordInsertTime(insertTime)
		errMsg := fmt.Sprintf("failed to insert %d new matrix jobs for job %d (JobID: %s): %v", len(newJobs), job.ID, job.JobID, err)
		log.Error("Matrix insertion error: %s", errMsg)
		GetMatrixMetrics().RecordReevaluation(time.Since(startTime), false, 0)
		return nil, fmt.Errorf("insert new jobs: %w", err)
	}
	insertTime := time.Since(insertStartTime)
	GetMatrixMetrics().RecordInsertTime(insertTime)

	// Mark the original job as evaluated
	job.IsMatrixEvaluated = true
	if _, err := actions_model.UpdateRunJob(ctx, job, nil, "is_matrix_evaluated"); err != nil {
		log.Error("Failed to update job %d is_matrix_evaluated flag: %v", job.ID, err)
	}

	totalTime := time.Since(startTime)
	GetMatrixMetrics().RecordReevaluation(totalTime, true, int64(len(newJobs)))

	log.Info("Successfully completed matrix re-evaluation for job %d (JobID: %s): created %d new jobs from %d matrix combinations (total: %dms, parse: %dms, insert: %dms)",
		job.ID, job.JobID, len(newJobs), len(jobs), totalTime.Milliseconds(), parseTime.Milliseconds(), insertTime.Milliseconds())

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

// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"

	"github.com/nektos/act/pkg/jobparser"
	"gopkg.in/yaml.v3"
)

// WorkflowEnvironmentInfo represents environment information extracted from a workflow
type WorkflowEnvironmentInfo struct {
	JobID       string
	Environment string
	URL         string // optional environment URL
}

// ExtractEnvironmentsFromWorkflow parses workflow content and extracts environment information
func ExtractEnvironmentsFromWorkflow(workflowContent []byte) ([]*WorkflowEnvironmentInfo, error) {
	var workflow struct {
		Jobs map[string]struct {
			Environment interface{} `yaml:"environment"`
		} `yaml:"jobs"`
	}

	if err := yaml.Unmarshal(workflowContent, &workflow); err != nil {
		return nil, err
	}

	var environments []*WorkflowEnvironmentInfo
	for jobID, job := range workflow.Jobs {
		if job.Environment == nil {
			continue
		}

		envInfo := &WorkflowEnvironmentInfo{JobID: jobID}

		switch env := job.Environment.(type) {
		case string:
			// Simple string format: environment: production
			envInfo.Environment = env
		case map[string]interface{}:
			// Object format: environment: { name: production, url: https://example.com }
			if name, ok := env["name"].(string); ok {
				envInfo.Environment = name
			}
			if url, ok := env["url"].(string); ok {
				envInfo.URL = url
			}
		}

		if envInfo.Environment != "" {
			environments = append(environments, envInfo)
		}
	}

	return environments, nil
}

// CreateDeploymentsForRun creates deployment records for a workflow run that targets environments
func CreateDeploymentsForRun(ctx context.Context, run *ActionRun, workflowContent []byte, jobs []*jobparser.SingleWorkflow) error {
	// Extract environment information from workflow
	envInfos, err := ExtractEnvironmentsFromWorkflow(workflowContent)
	if err != nil {
		log.Warn("Failed to extract environments from workflow: %v", err)
		return nil // Don't fail the run if environment parsing fails
	}

	if len(envInfos) == 0 {
		return nil // No environments specified
	}

	// Create deployments for each environment
	for _, envInfo := range envInfos {
		// Find or create the environment by name
		env, err := CreateOrGetEnvironmentByName(ctx, run.RepoID, envInfo.Environment, run.TriggerUserID, envInfo.URL)
		if err != nil {
			log.Error("Failed to create or get environment '%s' for repo %d: %v", envInfo.Environment, run.RepoID, err)
			continue // Skip if environment creation/retrieval fails
		}

		// Find the job name for this job ID
		var jobName string
		for _, job := range jobs {
			jobID, _ := job.Job()
			if strings.EqualFold(jobID, envInfo.JobID) {
				_, jobModel := job.Job()
				jobName = jobModel.Name
				break
			}
		}
		if jobName == "" {
			jobName = envInfo.JobID
		}

		// Create deployment record
		deployment := &ActionDeployment{
			RepoID:        run.RepoID,
			RunID:         run.ID,
			EnvironmentID: env.ID,
			Ref:           run.Ref,
			CommitSHA:     run.CommitSHA,
			Task:          jobName,
			Status:        DeploymentStatusQueued,
			Description:   "Deployment to " + envInfo.Environment + " environment",
			CreatedByID:   run.TriggerUserID,
		}

		if err := db.Insert(ctx, deployment); err != nil {
			log.Error("Failed to create deployment for environment '%s': %v", envInfo.Environment, err)
			continue
		}

		log.Info("Created deployment %d for run %d to environment '%s'", deployment.ID, run.ID, envInfo.Environment)
	}

	return nil
}

// UpdateDeploymentStatusForRun updates deployment status when workflow run status changes
func UpdateDeploymentStatusForRun(ctx context.Context, run *ActionRun) error {
	// Find deployments for this run
	deployments, err := FindDeployments(ctx, FindDeploymentsOptions{
		RunID: run.ID,
	})
	if err != nil {
		return err
	}

	if len(deployments) == 0 {
		return nil // No deployments to update
	}

	// Map run status to deployment status
	var deploymentStatus DeploymentStatus
	switch run.Status {
	case StatusWaiting, StatusBlocked:
		deploymentStatus = DeploymentStatusQueued
	case StatusRunning:
		deploymentStatus = DeploymentStatusInProgress
	case StatusSuccess:
		deploymentStatus = DeploymentStatusSuccess
	case StatusFailure, StatusCancelled:
		deploymentStatus = DeploymentStatusFailure
	default:
		return nil // Don't update for unknown statuses
	}

	// Update all deployments for this run
	for _, deployment := range deployments {
		if err := UpdateDeploymentStatus(ctx, deployment.ID, deploymentStatus, ""); err != nil {
			log.Error("Failed to update deployment %d status to %s: %v", deployment.ID, deploymentStatus, err)
		}
	}

	return nil
}

// InsertRunWithDeployments creates a run and automatically creates deployments for any environments specified
func InsertRunWithDeployments(ctx context.Context, run *ActionRun, jobs []*jobparser.SingleWorkflow, workflowContent []byte) error {
	// First create the run using the existing logic
	if err := InsertRun(ctx, run, jobs); err != nil {
		return err
	}

	// Then create deployments if environments are specified
	if err := CreateDeploymentsForRun(ctx, run, workflowContent, jobs); err != nil {
		log.Error("Failed to create deployments for run %d: %v", run.ID, err)
		// Don't fail the run creation if deployment creation fails
	}

	return nil
}

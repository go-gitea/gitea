// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// DeploymentStatus represents the deployment status
type DeploymentStatus string

const (
	DeploymentStatusQueued     DeploymentStatus = "queued"
	DeploymentStatusInProgress DeploymentStatus = "in_progress"
	DeploymentStatusSuccess    DeploymentStatus = "success"
	DeploymentStatusFailure    DeploymentStatus = "failure"
	DeploymentStatusCancelled  DeploymentStatus = "cancelled"
	DeploymentStatusError      DeploymentStatus = "error"
)

// ActionDeployment represents a deployment attempt to an environment
type ActionDeployment struct {
	ID            int64            `xorm:"pk autoincr"`
	RepoID        int64            `xorm:"INDEX NOT NULL"`
	Repo          *repo_model.Repository `xorm:"-"`
	RunID         int64            `xorm:"INDEX NOT NULL"`
	Run           *ActionRun       `xorm:"-"`
	EnvironmentID int64            `xorm:"INDEX NOT NULL"`
	Environment   *ActionEnvironment `xorm:"-"`
	
	// Deployment details
	Ref         string           `xorm:"INDEX"` // the commit/branch/tag being deployed
	CommitSHA   string           `xorm:"INDEX"` // the commit SHA being deployed
	Task        string           // deployment task/job name
	Status      DeploymentStatus `xorm:"INDEX"`
	Description string           `xorm:"TEXT"`
	LogURL      string           `xorm:"TEXT"`
	
	// Creator info
	CreatedByID int64              `xorm:"INDEX"`
	CreatedBy   *user_model.User   `xorm:"-"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionDeployment))
}

// TableName returns the table name for ActionDeployment
func (ActionDeployment) TableName() string {
	return "action_deployment"
}

// LoadEnvironment loads the environment for this deployment
func (d *ActionDeployment) LoadEnvironment(ctx context.Context) error {
	if d.Environment != nil {
		return nil
	}
	env := &ActionEnvironment{}
	has, err := db.GetEngine(ctx).ID(d.EnvironmentID).Get(env)
	if err != nil {
		return err
	}
	if has {
		d.Environment = env
	}
	return nil
}

// LoadRun loads the run for this deployment
func (d *ActionDeployment) LoadRun(ctx context.Context) error {
	if d.Run != nil {
		return nil
	}
	run := &ActionRun{}
	has, err := db.GetEngine(ctx).ID(d.RunID).Get(run)
	if err != nil {
		return err
	}
	if has {
		d.Run = run
	}
	return nil
}

// LoadRepo loads the repository for this deployment
func (d *ActionDeployment) LoadRepo(ctx context.Context) error {
	if d.Repo != nil {
		return nil
	}
	var err error
	d.Repo, err = repo_model.GetRepositoryByID(ctx, d.RepoID)
	return err
}

// LoadCreatedBy loads the user who created this deployment
func (d *ActionDeployment) LoadCreatedBy(ctx context.Context) error {
	if d.CreatedBy != nil {
		return nil
	}
	var err error
	d.CreatedBy, err = user_model.GetUserByID(ctx, d.CreatedByID)
	return err
}

// CreateDeploymentOptions contains options for creating a deployment
type CreateDeploymentOptions struct {
	RepoID        int64
	RunID         int64
	EnvironmentID int64
	Ref           string
	CommitSHA     string
	Task          string
	Description   string
	CreatedByID   int64
}

// CreateDeployment creates a new deployment
func CreateDeployment(ctx context.Context, opts CreateDeploymentOptions) (*ActionDeployment, error) {
	deployment := &ActionDeployment{
		RepoID:        opts.RepoID,
		RunID:         opts.RunID,
		EnvironmentID: opts.EnvironmentID,
		Ref:           opts.Ref,
		CommitSHA:     opts.CommitSHA,
		Task:          opts.Task,
		Status:        DeploymentStatusQueued,
		Description:   opts.Description,
		CreatedByID:   opts.CreatedByID,
	}

	return deployment, db.Insert(ctx, deployment)
}

// FindDeploymentsOptions contains options for finding deployments
type FindDeploymentsOptions struct {
	db.ListOptions
	RepoID        int64
	RunID         int64
	EnvironmentID int64
	Status        []DeploymentStatus
	Ref           string
}

func (opts FindDeploymentsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()

	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	if opts.RunID > 0 {
		cond = cond.And(builder.Eq{"run_id": opts.RunID})
	}

	if opts.EnvironmentID > 0 {
		cond = cond.And(builder.Eq{"environment_id": opts.EnvironmentID})
	}

	if len(opts.Status) > 0 {
		cond = cond.And(builder.In("status", opts.Status))
	}

	if opts.Ref != "" {
		cond = cond.And(builder.Eq{"ref": opts.Ref})
	}

	return cond
}

// FindDeployments finds deployments with the given options
func FindDeployments(ctx context.Context, opts FindDeploymentsOptions) ([]*ActionDeployment, error) {
	return db.Find[ActionDeployment](ctx, opts)
}

// UpdateDeploymentStatus updates the deployment status
func UpdateDeploymentStatus(ctx context.Context, deploymentID int64, status DeploymentStatus, logURL string) error {
	deployment := &ActionDeployment{
		Status: status,
		LogURL: logURL,
	}
	_, err := db.GetEngine(ctx).ID(deploymentID).Cols("status", "log_url", "updated_unix").Update(deployment)
	return err
}

// DeleteDeployment deletes a deployment
func DeleteDeployment(ctx context.Context, deploymentID int64) error {
	_, err := db.GetEngine(ctx).ID(deploymentID).Delete(&ActionDeployment{})
	return err
}

// CountDeployments counts deployments for a repository
func CountDeployments(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Where("repo_id = ?", repoID).Count(&ActionDeployment{})
}
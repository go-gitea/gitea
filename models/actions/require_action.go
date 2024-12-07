// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

type RequireAction struct {
	ID           int64              `xorm:"pk autoincr"`
	OrgID        int64              `xorm:"INDEX"`
	RepoName     string             `xorm:"VARCHAR(255)"`
	WorkflowName string             `xorm:"VARCHAR(255) UNIQUE(require_action) NOT NULL"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix  timeutil.TimeStamp `xorm:"updated"`
}

type GlobalWorkflow struct {
	RepoName string
	Filename string
}

func init() {
	db.RegisterModel(new(RequireAction))
}

type FindRequireActionOptions struct {
	db.ListOptions
	RequireActionID int64
	OrgID           int64
	RepoName        string
}

func (opts FindRequireActionOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OrgID > 0 {
		cond = cond.And(builder.Eq{"org_id": opts.OrgID})
	}
	if opts.RequireActionID > 0 {
		cond = cond.And(builder.Eq{"id": opts.RequireActionID})
	}
	if opts.RepoName != "" {
		cond = cond.And(builder.Eq{"repo_name": opts.RepoName})
	}
	return cond
}

// LoadAttributes loads the attributes of the require action
func (r *RequireAction) LoadAttributes(ctx context.Context) error {
	// place holder for now.
	return nil
}

// if the workflow is removable
func (r *RequireAction) Removable(orgID int64) bool {
	// everyone can remove for now
	return r.OrgID == orgID
}

func AddRequireAction(ctx context.Context, orgID int64, repoName, workflowName string) (*RequireAction, error) {
	ra := &RequireAction{
		OrgID:        orgID,
		RepoName:     repoName,
		WorkflowName: workflowName,
	}
	return ra, db.Insert(ctx, ra)
}

func DeleteRequireAction(ctx context.Context, requireActionID int64) error {
	if _, err := db.DeleteByID[RequireAction](ctx, requireActionID); err != nil {
		return err
	}
	return nil
}

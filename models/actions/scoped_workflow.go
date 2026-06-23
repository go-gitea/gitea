// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ActionScopedWorkflowSource registers a repository as a source of scoped workflows, either for an owner (user/org) or for the whole instance.
type ActionScopedWorkflowSource struct {
	ID int64 `xorm:"pk autoincr"`

	// OwnerID is the scope the source applies to: a user/org ID (applies to that owner's repos), or 0 for instance-level (applies to every repo).
	OwnerID int64 `xorm:"UNIQUE(owner_repo) NOT NULL DEFAULT 0"`
	// SourceRepoID is the source repository providing the workflow files; always non-zero.
	SourceRepoID int64 `xorm:"INDEX UNIQUE(owner_repo) NOT NULL DEFAULT 0"`

	// WorkflowConfigs maps a workflow ID (entry name) to its merge-gate config.
	WorkflowConfigs map[string]*ScopedWorkflowConfig `xorm:"JSON TEXT 'workflow_configs'"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// ScopedWorkflowConfig is one scoped workflow's config within a source registration.
type ScopedWorkflowConfig struct {
	Required bool     `json:"required"`
	Patterns []string `json:"patterns"` // the status-check patterns that must be present and pass, only effective when Required is true
}

func init() {
	db.RegisterModel(new(ActionScopedWorkflowSource))
}

// IsWorkflowRequired reports whether the given workflow ID (entry name) is marked required in this source.
func (s *ActionScopedWorkflowSource) IsWorkflowRequired(workflowID string) bool {
	c, ok := s.WorkflowConfigs[workflowID]
	return ok && c.Required
}

type FindScopedWorkflowSourceOpts struct {
	db.ListOptions
	OwnerIDs     []int64
	SourceRepoID int64
}

func (opts FindScopedWorkflowSourceOpts) ToConds() builder.Cond {
	cond := builder.NewCond()
	if len(opts.OwnerIDs) > 0 {
		cond = cond.And(builder.In("owner_id", opts.OwnerIDs))
	}
	if opts.SourceRepoID != 0 {
		cond = cond.And(builder.Eq{"source_repo_id": opts.SourceRepoID})
	}
	return cond
}

// GetEffectiveScopedWorkflowSources returns the scoped-workflow sources effective for a repo owned by repoOwnerID:
// the owner's own sources plus instance-level (owner_id=0) sources.
func GetEffectiveScopedWorkflowSources(ctx context.Context, repoOwnerID int64) ([]*ActionScopedWorkflowSource, error) {
	owners := []int64{0}
	if repoOwnerID != 0 {
		owners = append(owners, repoOwnerID)
	}
	return db.Find[ActionScopedWorkflowSource](ctx, FindScopedWorkflowSourceOpts{OwnerIDs: owners})
}

// IsScopedWorkflowSourceEffective reports whether sourceRepoID is a scoped-workflow source effective for a repo owned by repoOwnerID.
func IsScopedWorkflowSourceEffective(ctx context.Context, repoOwnerID, sourceRepoID int64) (bool, error) {
	owners := []int64{0}
	if repoOwnerID != 0 {
		owners = append(owners, repoOwnerID)
	}
	return db.Exist[ActionScopedWorkflowSource](ctx, FindScopedWorkflowSourceOpts{OwnerIDs: owners, SourceRepoID: sourceRepoID}.ToConds())
}

// IsWorkflowRequiredInSources reports whether workflowID from sourceRepoID is required by any of the given sources.
func IsWorkflowRequiredInSources(sources []*ActionScopedWorkflowSource, sourceRepoID int64, workflowID string) bool {
	for _, s := range sources {
		if s.SourceRepoID == sourceRepoID && s.IsWorkflowRequired(workflowID) {
			return true
		}
	}
	return false
}

// ScopedStatusContextPrefix returns the source-repo prefix that makes a scoped run's commit-status context distinct from same-named workflows.
func ScopedStatusContextPrefix(ctx context.Context, sourceRepoID int64) string {
	if sourceRepo, err := repo_model.GetRepositoryByID(ctx, sourceRepoID); err == nil {
		return sourceRepo.FullName()
	}
	return fmt.Sprintf("scoped:%d", sourceRepoID)
}

// IsScopedWorkflowRequired reports whether workflowID from sourceRepoID is required for a repo owned by consumerOwnerID.
func IsScopedWorkflowRequired(ctx context.Context, consumerOwnerID, sourceRepoID int64, workflowID string) (bool, error) {
	sources, err := GetEffectiveScopedWorkflowSources(ctx, consumerOwnerID)
	if err != nil {
		return false, err
	}
	return IsWorkflowRequiredInSources(sources, sourceRepoID, workflowID), nil
}

// GetScopedWorkflowSourcesByOwner returns the sources an owner (user/org, or 0 for instance) registered.
func GetScopedWorkflowSourcesByOwner(ctx context.Context, ownerID int64) ([]*ActionScopedWorkflowSource, error) {
	return db.Find[ActionScopedWorkflowSource](ctx, FindScopedWorkflowSourceOpts{OwnerIDs: []int64{ownerID}})
}

// GetScopedWorkflowSource returns the (owner, repo) source registration or a NotExist error.
func GetScopedWorkflowSource(ctx context.Context, ownerID, repoID int64) (*ActionScopedWorkflowSource, error) {
	src := &ActionScopedWorkflowSource{}
	has, err := db.GetEngine(ctx).Where("owner_id = ? AND source_repo_id = ?", ownerID, repoID).Get(src)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.NewNotExistErrorf("scoped workflow source (owner %d, repo %d) does not exist", ownerID, repoID)
	}
	return src, nil
}

// AddScopedWorkflowSource registers repoID as a source for ownerID (no-op if already registered).
func AddScopedWorkflowSource(ctx context.Context, ownerID, repoID int64) error {
	exists, err := db.GetEngine(ctx).Where("owner_id = ? AND source_repo_id = ?", ownerID, repoID).Exist(new(ActionScopedWorkflowSource))
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return db.Insert(ctx, &ActionScopedWorkflowSource{OwnerID: ownerID, SourceRepoID: repoID})
}

// SetScopedWorkflowSourceConfigs replaces the per-workflow merge-gate configs (workflow ID -> config).
func SetScopedWorkflowSourceConfigs(ctx context.Context, ownerID, repoID int64, configs map[string]*ScopedWorkflowConfig) error {
	_, err := db.GetEngine(ctx).Where("owner_id = ? AND source_repo_id = ?", ownerID, repoID).
		Cols("workflow_configs").
		Update(&ActionScopedWorkflowSource{WorkflowConfigs: configs})
	return err
}

// RemoveScopedWorkflowSource removes the (owner, repo) source registration.
func RemoveScopedWorkflowSource(ctx context.Context, ownerID, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("owner_id = ? AND source_repo_id = ?", ownerID, repoID).Delete(new(ActionScopedWorkflowSource))
	return err
}

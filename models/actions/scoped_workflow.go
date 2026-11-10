// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/glob"
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

	RepoFilter *ScopedWorkflowRepoFilter `xorm:"JSON TEXT 'repo_filter'"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

type ScopedWorkflowRepoFilter struct {
	IncludeRepos  []string `json:"include_repos,omitempty"`
	ExcludeRepos  []string `json:"exclude_repos,omitempty"`
	IncludeTopics []string `json:"include_topics,omitempty"`
}

func (s *ActionScopedWorkflowSource) AppliesToRepo(repo *repo_model.Repository) bool {
	f := s.RepoFilter
	if f == nil {
		return true
	}
	matches := func(patterns []string) bool {
		for _, pattern := range patterns {
			g, err := glob.Compile(strings.ToLower(pattern))
			if err == nil && g.Match(strings.ToLower(repo.Name)) {
				return true
			}
		}
		return false
	}
	if matches(f.ExcludeRepos) {
		return false
	}
	if len(f.IncludeRepos) == 0 && len(f.IncludeTopics) == 0 || matches(f.IncludeRepos) {
		return true
	}
	for _, topic := range repo.Topics {
		for _, included := range f.IncludeTopics {
			if strings.EqualFold(topic, included) {
				return true
			}
		}
	}
	return false
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

func (opts FindScopedWorkflowSourceOpts) ToOrders() string {
	return "id"
}

func (opts FindScopedWorkflowSourceOpts) ToConds() builder.Cond {
	cond := builder.NewCond()
	if len(opts.OwnerIDs) > 0 {
		cond = cond.And(builder.In("owner_id", opts.OwnerIDs))
	}
	if opts.SourceRepoID != 0 {
		cond = cond.And(builder.Eq{"source_repo_id": opts.SourceRepoID})
	}
	return cond.And(scopedWorkflowSourceOwnedCond())
}

func scopedWorkflowSourceOwnedCond() builder.Cond {
	return builder.Eq{"owner_id": 0}.Or(builder.Expr("EXISTS (SELECT 1 FROM repository WHERE repository.id = action_scoped_workflow_source.source_repo_id AND repository.owner_id = action_scoped_workflow_source.owner_id)"))
}

// GetEffectiveScopedWorkflowSources returns the scoped-workflow sources matching repo:
// the owner's own sources plus instance-level (owner_id=0) sources.
func GetEffectiveScopedWorkflowSources(ctx context.Context, repo *repo_model.Repository) ([]*ActionScopedWorkflowSource, error) {
	sources, err := db.Find[ActionScopedWorkflowSource](ctx, FindScopedWorkflowSourceOpts{OwnerIDs: []int64{0, repo.OwnerID}})
	if err != nil {
		return nil, err
	}
	effective := sources[:0]
	for _, source := range sources {
		if source.AppliesToRepo(repo) {
			effective = append(effective, source)
		}
	}
	return effective, nil
}

func IsScopedWorkflowSourceEffective(ctx context.Context, repo *repo_model.Repository, sourceRepoID int64) (bool, error) {
	sources, err := GetEffectiveScopedWorkflowSources(ctx, repo)
	if err != nil {
		return false, err
	}
	for _, source := range sources {
		if source.SourceRepoID == sourceRepoID {
			return true, nil
		}
	}
	return false, nil
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

// IsScopedWorkflowRequired reports whether workflowID from sourceRepoID is required for repo.
func IsScopedWorkflowRequired(ctx context.Context, repo *repo_model.Repository, sourceRepoID int64, workflowID string) (bool, error) {
	sources, err := GetEffectiveScopedWorkflowSources(ctx, repo)
	if err != nil {
		return false, err
	}
	return IsWorkflowRequiredInSources(sources, sourceRepoID, workflowID), nil
}

// IsScopedWorkflowOptedOutloads the consumer's effective sources then calls ScopedWorkflowOptedOut
func IsScopedWorkflowOptedOut(ctx context.Context, cfg *repo_model.ActionsConfig, repo *repo_model.Repository, sourceRepoID int64, workflowID string) (bool, error) {
	if !cfg.IsScopedWorkflowDisabled(sourceRepoID, workflowID) {
		return false, nil
	}
	sources, err := GetEffectiveScopedWorkflowSources(ctx, repo)
	if err != nil {
		return false, err
	}
	return ScopedWorkflowOptedOut(cfg, sources, sourceRepoID, workflowID), nil
}

// ScopedWorkflowOptedOut reports whether a consumer's opt-out of (sourceRepoID, workflowID) is in effect.
func ScopedWorkflowOptedOut(cfg *repo_model.ActionsConfig, sources []*ActionScopedWorkflowSource, sourceRepoID int64, workflowID string) bool {
	return !IsWorkflowRequiredInSources(sources, sourceRepoID, workflowID) && cfg.IsScopedWorkflowDisabled(sourceRepoID, workflowID)
}

// GetScopedWorkflowSourcesByOwner returns the sources an owner (user/org, or 0 for instance) registered.
func GetScopedWorkflowSourcesByOwner(ctx context.Context, ownerID int64) ([]*ActionScopedWorkflowSource, error) {
	return db.Find[ActionScopedWorkflowSource](ctx, FindScopedWorkflowSourceOpts{OwnerIDs: []int64{ownerID}})
}

// GetScopedWorkflowSource returns the (owner, repo) source registration or a NotExist error.
func GetScopedWorkflowSource(ctx context.Context, ownerID, repoID int64) (*ActionScopedWorkflowSource, error) {
	src, has, err := db.Get[ActionScopedWorkflowSource](ctx, builder.Eq{"owner_id": ownerID, "source_repo_id": repoID}.And(scopedWorkflowSourceOwnedCond()))
	if err != nil {
		return nil, err
	} else if !has {
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
	if err := db.Insert(ctx, &ActionScopedWorkflowSource{OwnerID: ownerID, SourceRepoID: repoID}); err != nil {
		// Re-check and treat an already-present row as the intended no-op.
		if exists, existErr := db.GetEngine(ctx).Where("owner_id = ? AND source_repo_id = ?", ownerID, repoID).Exist(new(ActionScopedWorkflowSource)); existErr == nil && exists {
			return nil
		}
		return err
	}
	return nil
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

// RemoveOwnerScopedWorkflowSources removes every owner-level registration of repoID, as only its current owner may hold one.
func RemoveOwnerScopedWorkflowSources(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("owner_id <> 0 AND source_repo_id = ?", repoID).Delete(new(ActionScopedWorkflowSource))
	return err
}

func SetScopedWorkflowSourceRepoFilter(ctx context.Context, ownerID, repoID int64, filter *ScopedWorkflowRepoFilter) error {
	_, err := db.GetEngine(ctx).Where("owner_id = ? AND source_repo_id = ?", ownerID, repoID).Cols("repo_filter").Update(&ActionScopedWorkflowSource{RepoFilter: filter})
	return err
}

func (f *ScopedWorkflowRepoFilter) Validate() error {
	for _, patterns := range [][]string{f.IncludeRepos, f.ExcludeRepos} {
		for _, pattern := range patterns {
			if _, err := glob.Compile(strings.ToLower(pattern)); err != nil {
				return err
			}
		}
	}
	return nil
}

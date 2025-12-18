// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
	"xorm.io/xorm/convert"
)

// ErrUnitTypeNotExist represents a "UnitTypeNotExist" kind of error.
type ErrUnitTypeNotExist struct {
	UT unit.Type
}

// IsErrUnitTypeNotExist checks if an error is a ErrUnitNotExist.
func IsErrUnitTypeNotExist(err error) bool {
	_, ok := err.(ErrUnitTypeNotExist)
	return ok
}

func (err ErrUnitTypeNotExist) Error() string {
	return "Unit type does not exist: " + err.UT.LogString()
}

func (err ErrUnitTypeNotExist) Unwrap() error {
	return util.ErrNotExist
}

// RepoUnit describes all units of a repository
type RepoUnit struct { //revive:disable-line:exported
	ID                  int64
	RepoID              int64              `xorm:"INDEX(s)"`
	Type                unit.Type          `xorm:"INDEX(s)"`
	Config              convert.Conversion `xorm:"TEXT"`
	CreatedUnix         timeutil.TimeStamp `xorm:"INDEX CREATED"`
	AnonymousAccessMode perm.AccessMode    `xorm:"NOT NULL DEFAULT 0"`
	EveryoneAccessMode  perm.AccessMode    `xorm:"NOT NULL DEFAULT 0"`
}

func init() {
	db.RegisterModel(new(RepoUnit))
}

// UnitConfig describes common unit config
type UnitConfig struct{}

// FromDB fills up a UnitConfig from serialized format.
func (cfg *UnitConfig) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports a UnitConfig to a serialized format.
func (cfg *UnitConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// ExternalWikiConfig describes external wiki config
type ExternalWikiConfig struct {
	ExternalWikiURL string
}

// FromDB fills up a ExternalWikiConfig from serialized format.
func (cfg *ExternalWikiConfig) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports a ExternalWikiConfig to a serialized format.
func (cfg *ExternalWikiConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// ExternalTrackerConfig describes external tracker config
type ExternalTrackerConfig struct {
	ExternalTrackerURL           string
	ExternalTrackerFormat        string
	ExternalTrackerStyle         string
	ExternalTrackerRegexpPattern string
}

// FromDB fills up a ExternalTrackerConfig from serialized format.
func (cfg *ExternalTrackerConfig) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports a ExternalTrackerConfig to a serialized format.
func (cfg *ExternalTrackerConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// IssuesConfig describes issues config
type IssuesConfig struct {
	EnableTimetracker                bool
	AllowOnlyContributorsToTrackTime bool
	EnableDependencies               bool
}

// FromDB fills up a IssuesConfig from serialized format.
func (cfg *IssuesConfig) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports a IssuesConfig to a serialized format.
func (cfg *IssuesConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// PullRequestsConfig describes pull requests config
type PullRequestsConfig struct {
	IgnoreWhitespaceConflicts     bool
	AllowMerge                    bool
	AllowRebase                   bool
	AllowRebaseMerge              bool
	AllowSquash                   bool
	AllowFastForwardOnly          bool
	AllowManualMerge              bool
	AutodetectManualMerge         bool
	AllowRebaseUpdate             bool
	DefaultDeleteBranchAfterMerge bool
	DefaultMergeStyle             MergeStyle
	DefaultAllowMaintainerEdit    bool
}

// FromDB fills up a PullRequestsConfig from serialized format.
func (cfg *PullRequestsConfig) FromDB(bs []byte) error {
	// AllowRebaseUpdate = true as default for existing PullRequestConfig in DB
	cfg.AllowRebaseUpdate = true
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports a PullRequestsConfig to a serialized format.
func (cfg *PullRequestsConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// IsMergeStyleAllowed returns if merge style is allowed
func (cfg *PullRequestsConfig) IsMergeStyleAllowed(mergeStyle MergeStyle) bool {
	return mergeStyle == MergeStyleMerge && cfg.AllowMerge ||
		mergeStyle == MergeStyleRebase && cfg.AllowRebase ||
		mergeStyle == MergeStyleRebaseMerge && cfg.AllowRebaseMerge ||
		mergeStyle == MergeStyleSquash && cfg.AllowSquash ||
		mergeStyle == MergeStyleFastForwardOnly && cfg.AllowFastForwardOnly ||
		mergeStyle == MergeStyleManuallyMerged && cfg.AllowManualMerge
}

// GetDefaultMergeStyle returns the default merge style for this pull request
func (cfg *PullRequestsConfig) GetDefaultMergeStyle() MergeStyle {
	if len(cfg.DefaultMergeStyle) != 0 {
		return cfg.DefaultMergeStyle
	}

	if setting.Repository.PullRequest.DefaultMergeStyle != "" {
		return MergeStyle(setting.Repository.PullRequest.DefaultMergeStyle)
	}

	return MergeStyleMerge
}

// ActionsTokenPermissionMode defines the default permission mode for Actions tokens
type ActionsTokenPermissionMode string

const (
	// ActionsTokenPermissionModePermissive - write access by default (current behavior, backwards compatible)
	ActionsTokenPermissionModePermissive ActionsTokenPermissionMode = "permissive"
	// ActionsTokenPermissionModeRestricted - read access by default
	ActionsTokenPermissionModeRestricted ActionsTokenPermissionMode = "restricted"
)

// ActionsTokenPermissions defines the permissions for different repository units
type ActionsTokenPermissions struct {
	// Contents (repository code) - read/write/none
	Contents perm.AccessMode `json:"contents"`
	// Issues - read/write/none
	Issues perm.AccessMode `json:"issues"`
	// PullRequests - read/write/none
	PullRequests perm.AccessMode `json:"pull_requests"`
	// Packages - read/write/none
	Packages perm.AccessMode `json:"packages"`
	// Actions - read/write/none
	Actions perm.AccessMode `json:"actions"`
	// Wiki - read/write/none
	Wiki perm.AccessMode `json:"wiki"`
}

// DefaultActionsTokenPermissions returns the default permissions for permissive mode
func DefaultActionsTokenPermissions(mode ActionsTokenPermissionMode) ActionsTokenPermissions {
	if mode == ActionsTokenPermissionModeRestricted {
		return ActionsTokenPermissions{
			Contents:     perm.AccessModeRead,
			Issues:       perm.AccessModeRead,
			PullRequests: perm.AccessModeRead,
			Packages:     perm.AccessModeRead,
			Actions:      perm.AccessModeRead,
			Wiki:         perm.AccessModeRead,
		}
	}
	// Permissive mode (default)
	return ActionsTokenPermissions{
		Contents:     perm.AccessModeWrite,
		Issues:       perm.AccessModeWrite,
		PullRequests: perm.AccessModeWrite,
		Packages:     perm.AccessModeRead, // Packages read by default for security
		Actions:      perm.AccessModeWrite,
		Wiki:         perm.AccessModeWrite,
	}
}

// ForkPullRequestPermissions returns the restricted permissions for fork pull requests
func ForkPullRequestPermissions() ActionsTokenPermissions {
	return ActionsTokenPermissions{
		Contents:     perm.AccessModeRead,
		Issues:       perm.AccessModeRead,
		PullRequests: perm.AccessModeRead,
		Packages:     perm.AccessModeRead,
		Actions:      perm.AccessModeRead,
		Wiki:         perm.AccessModeRead,
	}
}

type ActionsConfig struct {
	DisabledWorkflows []string
	// CollaborativeOwnerIDs is a list of owner IDs used to share actions from private repos.
	// Only workflows from the private repos whose owners are in CollaborativeOwnerIDs can access the current repo's actions.
	CollaborativeOwnerIDs []int64
	// TokenPermissionMode defines the default permission mode (permissive or restricted)
	TokenPermissionMode ActionsTokenPermissionMode `json:"token_permission_mode,omitempty"`
	// DefaultTokenPermissions defines the default permissions for workflow tokens
	DefaultTokenPermissions *ActionsTokenPermissions `json:"default_token_permissions,omitempty"`
	// MaxTokenPermissions defines the maximum permissions (cannot be exceeded by workflow permissions keyword)
	MaxTokenPermissions *ActionsTokenPermissions `json:"max_token_permissions,omitempty"`
	// AllowCrossRepoAccess indicates if actions in this repo/org can access other repos in the same org
	AllowCrossRepoAccess bool `json:"allow_cross_repo_access,omitempty"`
}

func (cfg *ActionsConfig) EnableWorkflow(file string) {
	cfg.DisabledWorkflows = util.SliceRemoveAll(cfg.DisabledWorkflows, file)
}

func (cfg *ActionsConfig) ToString() string {
	return strings.Join(cfg.DisabledWorkflows, ",")
}

func (cfg *ActionsConfig) IsWorkflowDisabled(file string) bool {
	return slices.Contains(cfg.DisabledWorkflows, file)
}

func (cfg *ActionsConfig) DisableWorkflow(file string) {
	if slices.Contains(cfg.DisabledWorkflows, file) {
		return
	}

	cfg.DisabledWorkflows = append(cfg.DisabledWorkflows, file)
}

func (cfg *ActionsConfig) AddCollaborativeOwner(ownerID int64) {
	if !slices.Contains(cfg.CollaborativeOwnerIDs, ownerID) {
		cfg.CollaborativeOwnerIDs = append(cfg.CollaborativeOwnerIDs, ownerID)
	}
}

func (cfg *ActionsConfig) RemoveCollaborativeOwner(ownerID int64) {
	cfg.CollaborativeOwnerIDs = util.SliceRemoveAll(cfg.CollaborativeOwnerIDs, ownerID)
}

func (cfg *ActionsConfig) IsCollaborativeOwner(ownerID int64) bool {
	return slices.Contains(cfg.CollaborativeOwnerIDs, ownerID)
}

// GetTokenPermissionMode returns the token permission mode (defaults to permissive for backwards compatibility)
func (cfg *ActionsConfig) GetTokenPermissionMode() ActionsTokenPermissionMode {
	if cfg.TokenPermissionMode == "" {
		return ActionsTokenPermissionModePermissive
	}
	return cfg.TokenPermissionMode
}

// GetEffectiveTokenPermissions returns the effective token permissions based on settings and context
func (cfg *ActionsConfig) GetEffectiveTokenPermissions(isForkPullRequest bool) ActionsTokenPermissions {
	// Fork pull requests always get restricted read-only access for security
	if isForkPullRequest {
		return ForkPullRequestPermissions()
	}

	// Use custom default permissions if set
	if cfg.DefaultTokenPermissions != nil {
		return *cfg.DefaultTokenPermissions
	}

	// Otherwise use mode-based defaults
	return DefaultActionsTokenPermissions(cfg.GetTokenPermissionMode())
}

// GetMaxTokenPermissions returns the maximum allowed permissions
func (cfg *ActionsConfig) GetMaxTokenPermissions() ActionsTokenPermissions {
	if cfg.MaxTokenPermissions != nil {
		return *cfg.MaxTokenPermissions
	}
	// Default max is write for everything except packages
	return ActionsTokenPermissions{
		Contents:     perm.AccessModeWrite,
		Issues:       perm.AccessModeWrite,
		PullRequests: perm.AccessModeWrite,
		Packages:     perm.AccessModeWrite,
		Actions:      perm.AccessModeWrite,
		Wiki:         perm.AccessModeWrite,
	}
}

// ClampPermissions ensures that the given permissions don't exceed the maximum
func (cfg *ActionsConfig) ClampPermissions(perms ActionsTokenPermissions) ActionsTokenPermissions {
	maxPerms := cfg.GetMaxTokenPermissions()
	return ActionsTokenPermissions{
		Contents:     min(perms.Contents, maxPerms.Contents),
		Issues:       min(perms.Issues, maxPerms.Issues),
		PullRequests: min(perms.PullRequests, maxPerms.PullRequests),
		Packages:     min(perms.Packages, maxPerms.Packages),
		Actions:      min(perms.Actions, maxPerms.Actions),
		Wiki:         min(perms.Wiki, maxPerms.Wiki),
	}
}

// FromDB fills up a ActionsConfig from serialized format.
func (cfg *ActionsConfig) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports a ActionsConfig to a serialized format.
func (cfg *ActionsConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// ProjectsMode represents the projects enabled for a repository
type ProjectsMode string

const (
	// ProjectsModeRepo allows only repo-level projects
	ProjectsModeRepo ProjectsMode = "repo"
	// ProjectsModeOwner allows only owner-level projects
	ProjectsModeOwner ProjectsMode = "owner"
	// ProjectsModeAll allows both kinds of projects
	ProjectsModeAll ProjectsMode = "all"
	// ProjectsModeNone doesn't allow projects
	ProjectsModeNone ProjectsMode = "none"
)

// ProjectsConfig describes projects config
type ProjectsConfig struct {
	ProjectsMode ProjectsMode
}

// FromDB fills up a ProjectsConfig from serialized format.
func (cfg *ProjectsConfig) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports a ProjectsConfig to a serialized format.
func (cfg *ProjectsConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

func (cfg *ProjectsConfig) GetProjectsMode() ProjectsMode {
	if cfg.ProjectsMode != "" {
		return cfg.ProjectsMode
	}

	return ProjectsModeAll
}

func (cfg *ProjectsConfig) IsProjectsAllowed(m ProjectsMode) bool {
	projectsMode := cfg.GetProjectsMode()

	if m == ProjectsModeNone {
		return true
	}

	return projectsMode == m || projectsMode == ProjectsModeAll
}

// BeforeSet is invoked from XORM before setting the value of a field of this object.
func (r *RepoUnit) BeforeSet(colName string, val xorm.Cell) {
	switch colName {
	case "type":
		switch unit.Type(db.Cell2Int64(val)) {
		case unit.TypeExternalWiki:
			r.Config = new(ExternalWikiConfig)
		case unit.TypeExternalTracker:
			r.Config = new(ExternalTrackerConfig)
		case unit.TypePullRequests:
			r.Config = new(PullRequestsConfig)
		case unit.TypeIssues:
			r.Config = new(IssuesConfig)
		case unit.TypeActions:
			r.Config = new(ActionsConfig)
		case unit.TypeProjects:
			r.Config = new(ProjectsConfig)
		case unit.TypeCode, unit.TypeReleases, unit.TypeWiki, unit.TypePackages:
			fallthrough
		default:
			r.Config = new(UnitConfig)
		}
	}
}

// Unit returns Unit
func (r *RepoUnit) Unit() unit.Unit {
	return unit.Units[r.Type]
}

// CodeConfig returns config for unit.TypeCode
func (r *RepoUnit) CodeConfig() *UnitConfig {
	return r.Config.(*UnitConfig)
}

// PullRequestsConfig returns config for unit.TypePullRequests
func (r *RepoUnit) PullRequestsConfig() *PullRequestsConfig {
	return r.Config.(*PullRequestsConfig)
}

// ReleasesConfig returns config for unit.TypeReleases
func (r *RepoUnit) ReleasesConfig() *UnitConfig {
	return r.Config.(*UnitConfig)
}

// ExternalWikiConfig returns config for unit.TypeExternalWiki
func (r *RepoUnit) ExternalWikiConfig() *ExternalWikiConfig {
	return r.Config.(*ExternalWikiConfig)
}

// IssuesConfig returns config for unit.TypeIssues
func (r *RepoUnit) IssuesConfig() *IssuesConfig {
	return r.Config.(*IssuesConfig)
}

// ExternalTrackerConfig returns config for unit.TypeExternalTracker
func (r *RepoUnit) ExternalTrackerConfig() *ExternalTrackerConfig {
	return r.Config.(*ExternalTrackerConfig)
}

// ActionsConfig returns config for unit.ActionsConfig
func (r *RepoUnit) ActionsConfig() *ActionsConfig {
	return r.Config.(*ActionsConfig)
}

// ProjectsConfig returns config for unit.ProjectsConfig
func (r *RepoUnit) ProjectsConfig() *ProjectsConfig {
	return r.Config.(*ProjectsConfig)
}

func getUnitsByRepoID(ctx context.Context, repoID int64) (units []*RepoUnit, err error) {
	var tmpUnits []*RepoUnit
	if err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Find(&tmpUnits); err != nil {
		return nil, err
	}

	for _, u := range tmpUnits {
		if !u.Type.UnitGlobalDisabled() {
			units = append(units, u)
		}
	}

	return units, nil
}

// UpdateRepoUnit updates the provided repo unit
func UpdateRepoUnit(ctx context.Context, unit *RepoUnit) error {
	_, err := db.GetEngine(ctx).ID(unit.ID).Update(unit)
	return err
}

func UpdateRepoUnitPublicAccess(ctx context.Context, unit *RepoUnit) error {
	_, err := db.GetEngine(ctx).Where("repo_id=? AND `type`=?", unit.RepoID, unit.Type).
		Cols("anonymous_access_mode", "everyone_access_mode").Update(unit)
	return err
}

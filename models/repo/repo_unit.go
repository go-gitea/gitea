// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
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
	return fmt.Sprintf("Unit type does not exist: %s", err.UT.String())
}

func (err ErrUnitTypeNotExist) Unwrap() error {
	return util.ErrNotExist
}

// RepoUnit describes all units of a repository
type RepoUnit struct { //revive:disable-line:exported
	ID          int64
	RepoID      int64              `xorm:"INDEX(s)"`
	Type        unit.Type          `xorm:"INDEX(s)"`
	Config      convert.Conversion `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
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

type ActionsConfig struct {
	DisabledWorkflows []string
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
	for _, workflow := range cfg.DisabledWorkflows {
		if file == workflow {
			return
		}
	}

	cfg.DisabledWorkflows = append(cfg.DisabledWorkflows, file)
}

// FromDB fills up a ActionsConfig from serialized format.
func (cfg *ActionsConfig) FromDB(bs []byte) error {
	return json.UnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports a ActionsConfig to a serialized format.
func (cfg *ActionsConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
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
		case unit.TypeCode, unit.TypeReleases, unit.TypeWiki, unit.TypeProjects, unit.TypePackages:
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

// UpdateRepositoryUnits updates a repository's units
func UpdateRepositoryUnits(ctx context.Context, repo *Repository, units []RepoUnit, deleteUnitTypes []unit.Type) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Delete existing settings of units before adding again
	for _, u := range units {
		deleteUnitTypes = append(deleteUnitTypes, u.Type)
	}

	if _, err = db.GetEngine(ctx).Where("repo_id = ?", repo.ID).In("type", deleteUnitTypes).Delete(new(RepoUnit)); err != nil {
		return err
	}

	if len(units) > 0 {
		if err = db.Insert(ctx, units); err != nil {
			return err
		}
	}

	return committer.Commit()
}

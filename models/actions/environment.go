// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ActionEnvironment represents a deployment environment
type ActionEnvironment struct {
	ID          int64  `xorm:"pk autoincr"`
	RepoID      int64  `xorm:"INDEX UNIQUE(repo_name) NOT NULL"`
	Name        string `xorm:"UNIQUE(repo_name) NOT NULL"`
	Description string `xorm:"TEXT"`
	ExternalURL string `xorm:"TEXT"`

	// Protection rules as JSON
	ProtectionRules string `xorm:"LONGTEXT"`

	// Audit fields
	CreatedByID int64              `xorm:"INDEX"`
	CreatedBy   *user_model.User   `xorm:"-"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`

	// Relationships
	Repo *repo_model.Repository `xorm:"-"`
}

const (
	EnvironmentNameMaxLength        = 255
	EnvironmentDescriptionMaxLength = 4096
	EnvironmentURLMaxLength         = 2048
	ProtectionRulesMaxLength        = 65536
)

func init() {
	db.RegisterModel(new(ActionEnvironment))
}

// TableName returns the table name for ActionEnvironment
func (ActionEnvironment) TableName() string {
	return "action_environment"
}

// LoadRepo loads the repository for this environment
func (env *ActionEnvironment) LoadRepo(ctx context.Context) error {
	if env.Repo != nil {
		return nil
	}
	var err error
	env.Repo, err = repo_model.GetRepositoryByID(ctx, env.RepoID)
	return err
}

// LoadCreatedBy loads the user who created this environment
func (env *ActionEnvironment) LoadCreatedBy(ctx context.Context) error {
	if env.CreatedBy != nil {
		return nil
	}
	var err error
	env.CreatedBy, err = user_model.GetUserByID(ctx, env.CreatedByID)
	return err
}

// CreateEnvironment creates a new deployment environment
func CreateEnvironment(ctx context.Context, opts CreateEnvironmentOptions) (*ActionEnvironment, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	env := &ActionEnvironment{
		RepoID:          opts.RepoID,
		Name:            opts.Name,
		Description:     opts.Description,
		ExternalURL:     opts.ExternalURL,
		ProtectionRules: opts.ProtectionRules,
		CreatedByID:     opts.CreatedByID,
	}

	return env, db.Insert(ctx, env)
}

// CreateEnvironmentOptions contains the options for creating an environment
type CreateEnvironmentOptions struct {
	RepoID          int64
	Name            string
	Description     string
	ExternalURL     string
	ProtectionRules string
	CreatedByID     int64
}

// Validate validates the create environment options
func (opts *CreateEnvironmentOptions) Validate() error {
	if opts.RepoID <= 0 {
		return util.NewInvalidArgumentErrorf("repository ID is required")
	}

	if strings.TrimSpace(opts.Name) == "" {
		return util.NewInvalidArgumentErrorf("environment name is required")
	}

	opts.Name = strings.TrimSpace(opts.Name)
	if utf8.RuneCountInString(opts.Name) > EnvironmentNameMaxLength {
		return util.NewInvalidArgumentErrorf("environment name too long")
	}

	if utf8.RuneCountInString(opts.Description) > EnvironmentDescriptionMaxLength {
		return util.NewInvalidArgumentErrorf("description too long")
	}

	if utf8.RuneCountInString(opts.ExternalURL) > EnvironmentURLMaxLength {
		return util.NewInvalidArgumentErrorf("external URL too long")
	}

	if utf8.RuneCountInString(opts.ProtectionRules) > ProtectionRulesMaxLength {
		return util.NewInvalidArgumentErrorf("protection rules too long")
	}

	opts.Description = util.TruncateRunes(opts.Description, EnvironmentDescriptionMaxLength)

	return nil
}

// FindEnvironmentsOptions contains options for finding environments
type FindEnvironmentsOptions struct {
	db.ListOptions
	RepoID int64
	Name   string
}

func (opts FindEnvironmentsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()

	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	if opts.Name != "" {
		cond = cond.And(builder.Eq{"name": opts.Name})
	}

	return cond
}

// FindEnvironments finds environments with the given options
func FindEnvironments(ctx context.Context, opts FindEnvironmentsOptions) ([]*ActionEnvironment, error) {
	return db.Find[ActionEnvironment](ctx, opts)
}

// GetEnvironmentByRepoIDAndName gets an environment by repository ID and name
func GetEnvironmentByRepoIDAndName(ctx context.Context, repoID int64, name string) (*ActionEnvironment, error) {
	env := &ActionEnvironment{}
	has, err := db.GetEngine(ctx).Where("repo_id = ? AND name = ?", repoID, name).Get(env)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.NewNotExistErrorf("environment does not exist")
	}
	return env, nil
}

// UpdateEnvironmentOptions contains options for updating an environment
type UpdateEnvironmentOptions struct {
	Description     *string
	ExternalURL     *string
	ProtectionRules *string
}

// Validate validates the update environment options
func (opts *UpdateEnvironmentOptions) Validate() error {
	if opts.Description != nil && utf8.RuneCountInString(*opts.Description) > EnvironmentDescriptionMaxLength {
		return util.NewInvalidArgumentErrorf("description too long")
	}

	if opts.ExternalURL != nil && utf8.RuneCountInString(*opts.ExternalURL) > EnvironmentURLMaxLength {
		return util.NewInvalidArgumentErrorf("external URL too long")
	}

	if opts.ProtectionRules != nil && utf8.RuneCountInString(*opts.ProtectionRules) > ProtectionRulesMaxLength {
		return util.NewInvalidArgumentErrorf("protection rules too long")
	}

	return nil
}

// UpdateEnvironment updates an environment
func UpdateEnvironment(ctx context.Context, env *ActionEnvironment, opts UpdateEnvironmentOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	cols := make([]string, 0)

	if opts.Description != nil {
		env.Description = util.TruncateRunes(*opts.Description, EnvironmentDescriptionMaxLength)
		cols = append(cols, "description")
	}

	if opts.ExternalURL != nil {
		env.ExternalURL = *opts.ExternalURL
		cols = append(cols, "external_url")
	}

	if opts.ProtectionRules != nil {
		env.ProtectionRules = *opts.ProtectionRules
		cols = append(cols, "protection_rules")
	}

	if len(cols) == 0 {
		return nil
	}

	cols = append(cols, "updated_unix")

	_, err := db.GetEngine(ctx).ID(env.ID).Cols(cols...).Update(env)
	return err
}

// DeleteEnvironment deletes an environment
func DeleteEnvironment(ctx context.Context, repoID int64, name string) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ? AND name = ?", repoID, name).Delete(&ActionEnvironment{})
	return err
}

// CountEnvironments counts environments for a repository
func CountEnvironments(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Where("repo_id = ?", repoID).Count(&ActionEnvironment{})
}

// CheckEnvironmentExists checks if an environment exists
func CheckEnvironmentExists(ctx context.Context, repoID int64, name string) (bool, error) {
	return db.GetEngine(ctx).Where("repo_id = ? AND name = ?", repoID, name).Exist(&ActionEnvironment{})
}

// CreateOrGetEnvironmentByName creates an environment if it doesn't exist, otherwise returns the existing one
func CreateOrGetEnvironmentByName(ctx context.Context, repoID int64, name string, createdByID int64, externalURL string) (*ActionEnvironment, error) {
	// First try to get existing environment
	env, err := GetEnvironmentByRepoIDAndName(ctx, repoID, name)
	if err == nil {
		return env, nil
	}

	// If not found, create a new environment with default values
	if errors.Is(err, util.ErrNotExist) {
		createOpts := CreateEnvironmentOptions{
			RepoID:          repoID,
			Name:            name,
			Description:     "Auto-created from Actions workflow",
			ExternalURL:     externalURL,
			ProtectionRules: "",
			CreatedByID:     createdByID,
		}

		return CreateEnvironment(ctx, createOpts)
	}

	// Return any other error
	return nil, err
}

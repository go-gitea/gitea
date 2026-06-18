// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	secret_model "gitea.dev/models/secret"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	secret_service "gitea.dev/services/secrets"
)

// CreateEnvironment creates a new deployment environment for a repository.
func CreateEnvironment(ctx context.Context, repoID int64, name, protectedBranches string) (*actions_model.ActionEnvironment, error) {
	if name == "" {
		return nil, util.NewInvalidArgumentErrorf("environment name cannot be empty")
	}
	if len(name) > 255 {
		return nil, util.NewInvalidArgumentErrorf("environment name too long")
	}
	return actions_model.InsertEnvironment(ctx, repoID, name, protectedBranches)
}

// UpdateEnvironment updates an existing environment.
func UpdateEnvironment(ctx context.Context, repoID, envID int64, name, protectedBranches string) (*actions_model.ActionEnvironment, error) {
	env, err := actions_model.GetEnvironmentByID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if env.RepoID != repoID {
		return nil, util.ErrNotExist
	}
	if name != "" && name != env.Name {
		if existing, err := actions_model.GetEnvironmentByRepoAndName(ctx, repoID, name); err == nil && existing != nil {
			return nil, actions_model.ErrEnvironmentAlreadyExists{Name: name}
		}
		env.Name = name
	}
	env.ProtectedBranches = protectedBranches
	return env, actions_model.UpdateEnvironment(ctx, env)
}

// DeleteEnvironment removes an environment and its scoped secrets/variables.
func DeleteEnvironment(ctx context.Context, repoID, envID int64) error {
	return actions_model.DeleteEnvironment(ctx, repoID, envID)
}

// CreateOrUpdateEnvSecret creates or updates a secret scoped to an environment.
func CreateOrUpdateEnvSecret(ctx context.Context, repoID, envID int64, name, data, description string) (*actions_model.ActionEnvironmentSecret, bool, error) {
	if err := secret_service.ValidateName(name); err != nil {
		return nil, false, err
	}
	if len(data) > secret_model.SecretDataMaxLength {
		return nil, false, util.NewInvalidArgumentErrorf("secret data too long")
	}

	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, data)
	if err != nil {
		return nil, false, err
	}

	existing, err := db.Find[actions_model.ActionEnvironmentSecret](ctx, actions_model.FindEnvSecretsOptions{
		RepoID:        repoID,
		EnvironmentID: envID,
		Name:          name,
	})
	if err != nil {
		return nil, false, err
	}

	if len(existing) == 0 {
		s := &actions_model.ActionEnvironmentSecret{
			RepoID:        repoID,
			EnvironmentID: envID,
			Name:          strings.ToUpper(name),
			Data:          encrypted,
			Description:   description,
		}
		return s, true, db.Insert(ctx, s)
	}

	s := existing[0]
	s.Data = encrypted
	s.Description = description
	_, err = db.GetEngine(ctx).ID(s.ID).Cols("data", "description").Update(s)
	return s, false, err
}

// DeleteEnvSecret removes a secret from an environment.
func DeleteEnvSecret(ctx context.Context, repoID, envID int64, name string) error {
	secrets, err := db.Find[actions_model.ActionEnvironmentSecret](ctx, actions_model.FindEnvSecretsOptions{
		RepoID:        repoID,
		EnvironmentID: envID,
		Name:          name,
	})
	if err != nil {
		return err
	}
	if len(secrets) == 0 {
		return util.ErrNotExist
	}
	_, err = db.DeleteByID[actions_model.ActionEnvironmentSecret](ctx, secrets[0].ID)
	return err
}

// CreateEnvVariable creates a variable scoped to an environment.
func CreateEnvVariable(ctx context.Context, repoID, envID int64, name, data, description string) (*actions_model.ActionEnvironmentVariable, error) {
	name = strings.ToUpper(name)
	if err := secret_service.ValidateName(name); err != nil {
		return nil, err
	}
	v := &actions_model.ActionEnvironmentVariable{
		RepoID:        repoID,
		EnvironmentID: envID,
		Name:          name,
		Data:          util.NormalizeStringEOL(data),
		Description:   description,
	}
	if err := db.Insert(ctx, v); err != nil {
		if isUniqueViolation(err) {
			return nil, actions_model.ErrEnvVariableAlreadyExists{Name: name}
		}
		return nil, err
	}
	return v, nil
}

// UpdateEnvVariable updates a variable scoped to an environment.
func UpdateEnvVariable(ctx context.Context, repoID, envID, varID int64, name, data, description string) (*actions_model.ActionEnvironmentVariable, error) {
	vars, err := db.Find[actions_model.ActionEnvironmentVariable](ctx, actions_model.FindEnvVariablesOptions{
		RepoID:        repoID,
		EnvironmentID: envID,
		VariableID:    varID,
	})
	if err != nil {
		return nil, err
	}
	if len(vars) == 0 {
		return nil, util.ErrNotExist
	}
	v := vars[0]
	if name != "" {
		if err := secret_service.ValidateName(name); err != nil {
			return nil, err
		}
		v.Name = strings.ToUpper(name)
	}
	v.Data = util.NormalizeStringEOL(data)
	v.Description = description
	_, err = db.GetEngine(ctx).ID(v.ID).Cols("name", "data", "description").Update(v)
	return v, err
}

// DeleteEnvVariable removes a variable from an environment.
func DeleteEnvVariable(ctx context.Context, repoID, envID, varID int64) error {
	vars, err := db.Find[actions_model.ActionEnvironmentVariable](ctx, actions_model.FindEnvVariablesOptions{
		RepoID:        repoID,
		EnvironmentID: envID,
		VariableID:    varID,
	})
	if err != nil {
		return err
	}
	if len(vars) == 0 {
		return util.ErrNotExist
	}
	_, err = db.DeleteByID[actions_model.ActionEnvironmentVariable](ctx, vars[0].ID)
	return err
}

// isUniqueViolation reports whether err is a database unique-constraint violation.
func isUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || // MySQL
		strings.Contains(msg, "duplicate key") || // PostgreSQL
		strings.Contains(msg, "UNIQUE constraint") // SQLite
}

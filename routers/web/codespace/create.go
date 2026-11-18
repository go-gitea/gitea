// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"html/template"
	"net/http"
	"slices"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

const tplCodespaceCreateConfirm templates.TplName = "codespace/create_confirm"

type createPermissionRepository struct {
	FullName    string
	Permissions []codespace_service.CreatePermissionRequest
}

type createConfigurationOption struct {
	Selection  string
	Name       string
	ScopeLabel string
}

type createSecretField struct {
	Name        string
	Description string
	Source      string
	Available   bool
	Recommended bool
}

type createEnvironmentOption struct {
	Tag         string
	ScopeLabels []string
	Online      bool
}

// New renders the creation confirmation for a repository and ref.
func New(ctx *context.Context) {
	if ctx.Doer == nil || ctx.Repo == nil || ctx.Repo.Repository == nil {
		ctx.NotFound()
		return
	}
	opts := codespace_service.CreateCodespaceOptions{
		User:                  ctx.Doer,
		Repo:                  ctx.Repo.Repository,
		RefType:               ctx.FormString("ref_type"),
		RefName:               ctx.FormString("ref_name"),
		DevContainerSelection: ctx.FormString("dev_container"),
		EnvironmentTag:        ctx.FormString("environment_tag"),
	}
	plan, err := codespace_service.PrepareCodespace(ctx, opts)
	if err != nil {
		if plan != nil && errors.Is(err, codespace_service.ErrCreateConfigurationInvalid) {
			renderCreateConfirm(ctx, http.StatusUnprocessableEntity, plan, opts, ctx.Tr("codespace.configuration_invalid"))
			return
		}
		handleCreateError(ctx, err)
		return
	}
	renderCreateConfirm(ctx, http.StatusOK, plan, opts, "")
}

// Create creates a Codespace after the user reviews the current plan.
func Create(ctx *context.Context) {
	if ctx.Doer == nil || ctx.Repo == nil || ctx.Repo.Repository == nil {
		ctx.NotFound()
		return
	}
	opts := codespace_service.CreateCodespaceOptions{
		User:                  ctx.Doer,
		Repo:                  ctx.Repo.Repository,
		RefType:               ctx.FormString("ref_type"),
		RefName:               ctx.FormString("ref_name"),
		DevContainerSelection: ctx.FormString("dev_container"),
		EnvironmentTag:        ctx.FormString("environment_tag"),
		RequestHash:           ctx.FormString("request_hash"),
	}
	plan, err := codespace_service.PrepareCodespace(ctx, opts)
	if err != nil {
		if plan != nil && errors.Is(err, codespace_service.ErrCreateConfigurationInvalid) {
			renderCreateConfirm(ctx, http.StatusUnprocessableEntity, plan, opts, ctx.Tr("codespace.configuration_invalid"))
			return
		}
		handleCreateError(ctx, err)
		return
	}
	opts.PermissionGrants = make(map[string]string, len(plan.Permissions))
	for _, permission := range plan.Permissions {
		opts.PermissionGrants[permission.FormName] = ctx.FormString(permission.FormName)
	}
	names := ctx.Req.PostForm["secret_name"]
	sources := ctx.Req.PostForm["secret_source"]
	values := ctx.Req.PostForm["secret_value"]
	if len(names) != len(sources) || len(names) != len(values) || len(names) > codespace_service.MaxCreateSecrets {
		renderCreateConfirm(ctx, http.StatusUnprocessableEntity, plan, opts, ctx.Tr("codespace.secret_create_invalid"))
		return
	}
	opts.Secrets = make([]codespace_service.CreateSecretInput, 0, len(names))
	for i, name := range names {
		opts.Secrets = append(opts.Secrets, codespace_service.CreateSecretInput{
			Name: name, Source: sources[i], Value: values[i],
		})
	}
	result, err := codespace_service.CreateCodespace(ctx, opts)
	if err != nil {
		if errors.Is(err, codespace_service.ErrCreateConfigurationInvalid) {
			plan.RequestHash = ""
			renderCreateConfirm(ctx, http.StatusUnprocessableEntity, plan, opts, ctx.Tr("codespace.configuration_invalid"))
			return
		}
		if errors.Is(err, codespace_service.ErrCreateEnvironmentUnavailable) || errors.Is(err, codespace_service.ErrCreateRequestChanged) {
			if currentPlan, prepareErr := codespace_service.PrepareCodespace(ctx, opts); prepareErr == nil {
				plan = currentPlan
			}
			errorMessage := ctx.Tr("codespace.error.environment_unavailable")
			if errors.Is(err, codespace_service.ErrCreateRequestChanged) {
				errorMessage = ctx.Tr("codespace.error.create_request_changed")
			}
			renderCreateConfirm(ctx, http.StatusUnprocessableEntity, plan, opts, errorMessage)
			return
		}
		errorKey := "codespace.error.invalid_create_request"
		switch {
		case errors.Is(err, codespace_service.ErrCreateSecretUnavailable):
			errorKey = "codespace.secret_saved_unavailable"
		case errors.Is(err, codespace_service.ErrUserSecretNameInvalid):
			errorKey = "codespace.secret_name_invalid"
		case errors.Is(err, codespace_service.ErrUserSecretNameConflict):
			errorKey = "codespace.secret_name_conflict"
		case errors.Is(err, codespace_service.ErrUserSecretValueInvalid):
			errorKey = "codespace.secret_value_invalid"
		case errors.Is(err, codespace_service.ErrUserSecretCountLimit):
			errorKey = "codespace.secret_count_limit"
		case errors.Is(err, codespace_service.ErrCreateSecretSizeLimit):
			errorKey = "codespace.secret_size_limit"
		}
		renderCreateConfirm(ctx, http.StatusUnprocessableEntity, plan, opts, ctx.Tr(errorKey))
		return
	}
	ctx.Redirect(setting.AppSubURL+codespaceDetailPath(result.CodespaceID), http.StatusSeeOther)
}

func renderCreateConfirm(ctx *context.Context, status int, plan *codespace_service.CreateCodespacePlan, opts codespace_service.CreateCodespaceOptions, errorMessage template.HTML) {
	ctx.RespHeader().Set("Cache-Control", "no-store")
	globalScopeLabel := ctx.Locale.TrString("codespace.scope.global")
	personalScopeLabel := ctx.Locale.TrString("codespace.scope.personal")
	environmentOptions := make([]createEnvironmentOption, 0, len(plan.Environments))
	for _, environment := range plan.Environments {
		scopeLabels := make([]string, 0, 2)
		if environment.Site {
			scopeLabels = append(scopeLabels, globalScopeLabel)
		}
		if environment.Personal {
			scopeLabels = append(scopeLabels, personalScopeLabel)
		}
		environmentOption := createEnvironmentOption{
			Tag:         environment.Tag,
			ScopeLabels: scopeLabels,
			Online:      environment.Online,
		}
		environmentOptions = append(environmentOptions, environmentOption)
		if environment.Selected {
			ctx.Data["CreateSelectedEnvironment"] = environmentOption
		}
	}
	permissionRepositories := make([]createPermissionRepository, 0)
	permissionGrants := make(map[string]string, len(plan.Permissions))
	for _, permission := range plan.Permissions {
		if len(permissionRepositories) == 0 || permissionRepositories[len(permissionRepositories)-1].FullName != permission.RepositoryFullName {
			permissionRepositories = append(permissionRepositories, createPermissionRepository{FullName: permission.RepositoryFullName})
		}
		index := len(permissionRepositories) - 1
		permissionRepositories[index].Permissions = append(permissionRepositories[index].Permissions, permission)
		permissionGrants[permission.FormName] = permission.ModeName
		if value := opts.PermissionGrants[permission.FormName]; value != "" {
			permissionGrants[permission.FormName] = value
		}
	}
	secrets := opts.Secrets
	if secrets == nil {
		secrets = make([]codespace_service.CreateSecretInput, 0, len(plan.RecommendedSecrets))
		for _, recommendation := range plan.RecommendedSecrets {
			secrets = append(secrets, codespace_service.CreateSecretInput{Name: recommendation.Name, Source: codespace_service.SecretSourceNone})
		}
	}
	recommendations := make(map[string]string, len(plan.RecommendedSecrets))
	for _, recommendation := range plan.RecommendedSecrets {
		recommendations[recommendation.Name] = recommendation.Description
	}
	secretFields := make([]createSecretField, 0, len(secrets))
	for _, secret := range secrets {
		name := strings.ToUpper(strings.TrimSpace(secret.Name))
		description, recommended := recommendations[name]
		secretFields = append(secretFields, createSecretField{
			Name: name, Description: description, Source: secret.Source,
			Available: slices.Contains(plan.AvailableSecrets, name), Recommended: recommended,
		})
	}

	ctx.Data["Title"] = ctx.Tr("codespace.confirm_create")
	ctx.Data["CreatePlan"] = plan
	ctx.Data["CreateEnvironmentOptions"] = environmentOptions
	ctx.Data["CreatePermissionRepositories"] = permissionRepositories
	ctx.Data["CreatePermissionGrants"] = permissionGrants
	ctx.Data["CreateSecretFields"] = secretFields
	ctx.Data["CreateSecretTemplate"] = createSecretField{Source: codespace_service.SecretSourceNone}
	ctx.Data["CreateEnvironmentExplicit"] = opts.EnvironmentTag != ""
	ctx.Data["CreateSecretsNeedReentry"] = opts.Secrets != nil
	ctx.Data["CreateError"] = errorMessage
	configurationOptions := make([]createConfigurationOption, 0, len(plan.DevContainerOptions))
	for _, option := range plan.DevContainerOptions {
		var scopeLabel string
		switch option.Scope {
		case "repository":
			scopeLabel = ctx.Locale.TrString("codespace.scope.repository")
		case "personal":
			scopeLabel = personalScopeLabel
		default:
			scopeLabel = globalScopeLabel
		}
		configurationOption := createConfigurationOption{
			Selection:  option.Selection,
			Name:       option.Name,
			ScopeLabel: scopeLabel,
		}
		configurationOptions = append(configurationOptions, configurationOption)
		if option.Selected {
			ctx.Data["CreateSelectedConfiguration"] = configurationOption
		}
	}
	ctx.Data["CreateConfigurationOptions"] = configurationOptions
	ctx.HTML(status, tplCodespaceCreateConfirm)
}

func handleCreateError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrCreatePermissionDenied):
		ctx.Flash.Error(ctx.Tr("codespace.error.permission_denied"))
	case errors.Is(err, codespace_service.ErrCreateStateUnavailable):
		ctx.Flash.Error(ctx.Tr("codespace.error.state_unavailable"))
	default:
		ctx.Flash.Error(ctx.Tr("codespace.error.invalid_create_request"))
	}
	if ctx.Repo != nil && ctx.Repo.Repository != nil {
		ctx.Redirect(ctx.Repo.RepoLink, http.StatusSeeOther)
		return
	}
	ctx.Redirect(setting.AppSubURL+"/-/codespaces", http.StatusSeeOther)
}

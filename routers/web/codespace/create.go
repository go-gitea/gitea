// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"html/template"
	"net/http"

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

// RepositoryRedirect redirects repository Codespace collection reads to the repository code page.
func RepositoryRedirect(ctx *context.Context) {
	if ctx.Repo == nil || ctx.Repo.Repository == nil {
		ctx.NotFound(nil)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink, http.StatusSeeOther)
}

// New renders the creation confirmation for a repository and ref.
func New(ctx *context.Context) {
	if ctx.Doer == nil || ctx.Repo == nil || ctx.Repo.Repository == nil {
		ctx.NotFound(nil)
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
		handleCreateError(ctx, err)
		return
	}
	renderCreateConfirm(ctx, http.StatusOK, plan, opts, "")
}

// Create creates a Codespace after the user reviews the current plan.
func Create(ctx *context.Context) {
	if ctx.Doer == nil || ctx.Repo == nil || ctx.Repo.Repository == nil {
		ctx.NotFound(nil)
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
		handleCreateError(ctx, err)
		return
	}
	opts.PermissionGrants = make(map[string]string, len(plan.Permissions))
	for _, permission := range plan.Permissions {
		opts.PermissionGrants[permission.FormName] = ctx.FormString(permission.FormName)
	}
	opts.RecommendedSecretValues = make(map[string]string, len(plan.RecommendedSecrets))
	opts.RecommendedSecretEnabled = make(map[string]bool, len(plan.RecommendedSecrets))
	for _, secret := range plan.RecommendedSecrets {
		opts.RecommendedSecretValues[secret.Name] = ctx.FormString("recommended_secret_value_" + secret.Name)
		opts.RecommendedSecretEnabled[secret.Name] = ctx.FormBool("recommended_secret_enable_" + secret.Name)
	}
	result, err := codespace_service.CreateCodespace(ctx, opts)
	if err != nil {
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
		handleCreateError(ctx, err)
		return
	}
	ctx.Redirect(setting.AppSubURL+codespaceDetailPath(result.CodespaceID), http.StatusSeeOther)
}

func renderCreateConfirm(ctx *context.Context, status int, plan *codespace_service.CreateCodespacePlan, opts codespace_service.CreateCodespaceOptions, errorMessage template.HTML) {
	ctx.RespHeader().Set("Cache-Control", "no-store")
	selectedEnvironment := ""
	for _, environment := range plan.Environments {
		if environment.Selected {
			selectedEnvironment = environment.Tag
			break
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
	secretEnabled := make(map[string]bool, len(plan.RecommendedSecrets))
	hasPendingRecommendedSecret := false
	for _, secret := range plan.RecommendedSecrets {
		secretEnabled[secret.Name] = opts.RecommendedSecretEnabled[secret.Name]
		hasPendingRecommendedSecret = hasPendingRecommendedSecret || !secret.Available
	}

	ctx.Data["Title"] = ctx.Tr("codespace.confirm_create")
	ctx.Data["CreatePlan"] = plan
	ctx.Data["CreateSelectedEnvironment"] = selectedEnvironment
	ctx.Data["CreatePermissionRepositories"] = permissionRepositories
	ctx.Data["CreatePermissionGrants"] = permissionGrants
	ctx.Data["CreateRecommendedSecretEnabled"] = secretEnabled
	ctx.Data["CreateHasPendingRecommendedSecret"] = hasPendingRecommendedSecret
	ctx.Data["CreateError"] = errorMessage
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

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"strings"

	github_model "gitea.dev/models/github"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	tplSettingsGitHubApps templates.TplName = "user/settings/github_apps"
)

// GitHubApps renders the GitHub App credentials management page
func GitHubApps(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings.github_apps")
	ctx.Data["PageIsSettingsGitHubApps"] = true

	loadGitHubAppsData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsGitHubApps)
}

// GitHubAppsPost handles adding a new GitHub App credential
func GitHubAppsPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewGitHubAppCredentialForm)
	ctx.Data["Title"] = ctx.Tr("settings.github_apps")
	ctx.Data["PageIsSettingsGitHubApps"] = true

	if ctx.HasError() {
		loadGitHubAppsData(ctx)
		ctx.HTML(http.StatusOK, tplSettingsGitHubApps)
		return
	}

	// Normalize and validate the private key
	privateKey := strings.TrimSpace(form.PrivateKey)

	// Ensure the private key has proper PEM format
	if !strings.HasPrefix(privateKey, "-----BEGIN") {
		ctx.Flash.Error(ctx.Tr("settings.github_app_invalid_private_key"))
		loadGitHubAppsData(ctx)
		ctx.HTML(http.StatusOK, tplSettingsGitHubApps)
		return
	}

	if !strings.HasSuffix(privateKey, "-----") {
		ctx.Flash.Error(ctx.Tr("settings.github_app_invalid_private_key"))
		loadGitHubAppsData(ctx)
		ctx.HTML(http.StatusOK, tplSettingsGitHubApps)
		return
	}

	// Encrypt the private key
	encryptedKey, err := secret.EncryptSecret(setting.SecretKey, privateKey)
	if err != nil {
		ctx.ServerError("EncryptSecret", err)
		return
	}

	// Set default base URL if empty
	baseURL := form.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}

	cred := &github_model.AppCredential{
		OwnerID:             ctx.Doer.ID,
		Name:                form.Name,
		ClientID:            form.ClientID,
		InstallationID:      form.InstallationID,
		PrivateKeyEncrypted: encryptedKey,
		BaseURL:             baseURL,
	}

	if err := github_model.CreateGithubAppCredential(ctx, cred); err != nil {
		ctx.ServerError("CreateGithubAppCredential", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.add_github_app_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/github_apps")
}

// DeleteGitHubApp handles deleting a GitHub App credential
func DeleteGitHubApp(ctx *context.Context) {
	id := ctx.FormInt64("id")

	// Check ownership
	owned, err := github_model.CheckGithubAppCredentialOwnership(ctx, id, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("CheckGithubAppCredentialOwnership", err)
		return
	}
	if !owned {
		ctx.NotFound(nil)
		return
	}

	if err := github_model.DeleteGithubAppCredential(ctx, id); err != nil {
		ctx.Flash.Error("DeleteGithubAppCredential: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.delete_github_app_success"))
	}

	ctx.JSONRedirect(setting.AppSubURL + "/user/settings/github_apps")
}

func loadGitHubAppsData(ctx *context.Context) {
	creds, err := github_model.GetGithubAppCredentialsByOwnerID(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.ServerError("GetGithubAppCredentialsByOwnerID", err)
		return
	}
	ctx.Data["Credentials"] = creds

	// Load repositories using each credential
	credentialRepos := make(map[int64][]*github_model.MirrorWithRepo)
	for _, cred := range creds {
		mirrors, err := github_model.GetMirrorsWithRepoByCredentialID(ctx, cred.ID)
		if err != nil {
			ctx.ServerError("GetMirrorsWithRepoByCredentialID", err)
			return
		}
		credentialRepos[cred.ID] = mirrors
	}
	ctx.Data["CredentialRepos"] = credentialRepos
}

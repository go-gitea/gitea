// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	secret_model "gitea.dev/models/secret"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
	secret_service "gitea.dev/services/secrets"
)

func SetSecretsContext(ctx *context.Context, ownerID, repoID int64) {
	secrets, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{OwnerID: ownerID, RepoID: repoID})
	if err != nil {
		ctx.ServerError("FindSecrets", err)
		return
	}

	ctx.Data["Secrets"] = secrets
	ctx.Data["DataMaxLength"] = secret_model.SecretDataMaxLength
	ctx.Data["DescriptionMaxLength"] = secret_model.SecretDescriptionMaxLength
}

func PerformSecretsPost(ctx *context.Context, owner *user_model.User, repo *repo_model.Repository, redirectURL string) {
	form := web.GetForm(ctx).(*forms.AddSecretForm)

	s, _, err := secret_service.CreateOrUpdateSecret(ctx, ctx.Doer, owner, repo, form.Name, util.NormalizeStringEOL(form.Data), form.Description)
	if err != nil {
		log.Error("CreateOrUpdateSecret failed: %v", err)
		ctx.JSONError(ctx.Tr("secrets.save_failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("secrets.save_success", s.Name))
	ctx.JSONRedirect(redirectURL)
}

func PerformSecretsDelete(ctx *context.Context, owner *user_model.User, repo *repo_model.Repository, redirectURL string) {
	id := ctx.FormInt64("id")

	err := secret_service.DeleteSecretByID(ctx, ctx.Doer, owner, repo, id)
	if err != nil {
		log.Error("DeleteSecretByID(%d) failed: %v", id, err)
		ctx.JSONError(ctx.Tr("secrets.deletion.failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("secrets.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}

// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	repo_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/shared/actions"
	"code.gitea.io/gitea/services/forms"
	secret_service "code.gitea.io/gitea/services/secrets"
)

func SetSecretsContext(ctx *context.Context, owner *user_model.User, repo *repo_model.Repository) {
	ownerID := int64(0)
	if owner != nil {
		ownerID = owner.ID
	}
	repoID := int64(0)
	if repo != nil {
		repoID = repo.ID
	}

	secrets, err := secret_model.FindSecrets(ctx, secret_model.FindSecretsOptions{OwnerID: ownerID, RepoID: repoID})
	if err != nil {
		ctx.ServerError("FindSecrets", err)
		return
	}

	ctx.Data["Secrets"] = secrets
}

func PerformSecretsPost(ctx *context.Context, doer, owner *user_model.User, repo *repo_model.Repository, redirectURL string) {
	form := web.GetForm(ctx).(*forms.AddSecretForm)

	s, _, err := secret_service.CreateOrUpdateSecret(ctx, doer, owner, repo, form.Name, actions.ReserveLineBreakForTextarea(form.Data))
	if err != nil {
		log.Error("CreateOrUpdateSecret failed: %v", err)
		ctx.JSONError(ctx.Tr("secrets.creation.failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("secrets.creation.success", s.Name))
	ctx.JSONRedirect(redirectURL)
}

func PerformSecretsDelete(ctx *context.Context, doer, owner *user_model.User, repo *repo_model.Repository, redirectURL string) {
	id := ctx.FormInt64("id")

	err := secret_service.DeleteSecretByID(ctx, doer, owner, repo, id)
	if err != nil {
		log.Error("DeleteSecretByID(%d) failed: %v", id, err)
		ctx.JSONError(ctx.Tr("secrets.deletion.failed"))
		return
	}

	ctx.Flash.Success(ctx.Tr("secrets.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}

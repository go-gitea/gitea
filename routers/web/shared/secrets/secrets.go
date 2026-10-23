// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	actions_model "codenest.dev/models/actions"
	"codenest.dev/models/db"
	repo_model "codenest.dev/models/repo"
	secret_model "codenest.dev/models/secret"
	user_model "codenest.dev/models/user"
	"codenest.dev/modules/log"
	"codenest.dev/modules/util"
	"codenest.dev/modules/web"
	"codenest.dev/services/audit"
	"codenest.dev/services/context"
	"codenest.dev/services/forms"
	secret_service "codenest.dev/services/secrets"
)

func SetSecretsContext(ctx *context.Context, scope actions_model.BelongingScope) {
	secrets, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{BelongingScope: scope})
	if err != nil {
		ctx.ServerError("FindSecrets", err)
		return
	}

	ctx.Data["Secrets"] = secrets
	ctx.Data["DataMaxLength"] = secret_model.SecretDataMaxLength
	ctx.Data["DescriptionMaxLength"] = secret_model.SecretDescriptionMaxLength
}

func PerformSecretsPost(ctx *context.Context, scope actions_model.BelongingScope, owner *user_model.User, repo *repo_model.Repository, redirectURL string) {
	form := web.GetForm[*forms.AddSecretForm](ctx)

	s, created, err := secret_service.CreateOrUpdateSecret(ctx, scope, form.Name, util.NormalizeStringEOL(form.Data), form.Description)
	if err != nil {
		ctx.JSONErrorAuto(err)
		return
	}

	actions := audit.SecretUpdate
	if created {
		actions = audit.SecretAdd
	}
	audit.RecordScoped(ctx, owner, repo, actions, "secret", s.Name)

	ctx.Flash.Success(ctx.Tr("secrets.save_success", s.Name))
	ctx.JSONRedirect(redirectURL)
}

func PerformSecretsDelete(ctx *context.Context, scope actions_model.BelongingScope, owner *user_model.User, repo *repo_model.Repository, redirectURL string) {
	id := ctx.FormInt64("id")

	s, err := secret_service.DeleteSecretByID(ctx, scope, id)
	if err != nil {
		log.Error("DeleteSecretByID(%d) failed: %v", id, err)
		ctx.JSONError(ctx.Tr("secrets.deletion.failed"))
		return
	}

	audit.RecordScoped(ctx, owner, repo, audit.SecretRemove, "secret", s.Name)

	ctx.Flash.Success(ctx.Tr("secrets.deletion.success"))
	ctx.JSONRedirect(redirectURL)
}

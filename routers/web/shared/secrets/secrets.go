// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/audit"
	"code.gitea.io/gitea/services/forms"
)

func SetSecretsContext(ctx *context.Context, ownerID, repoID int64) {
	secrets, err := secret_model.FindSecrets(ctx, secret_model.FindSecretsOptions{OwnerID: ownerID, RepoID: repoID})
	if err != nil {
		ctx.ServerError("FindSecrets", err)
		return
	}

	ctx.Data["Secrets"] = secrets
}

func PerformSecretsPost(ctx *context.Context, doer, owner *user_model.User, repo *repo_model.Repository, redirectURL string) {
	form := web.GetForm(ctx).(*forms.AddSecretForm)

	s, err := secret_model.InsertEncryptedSecret(ctx, tryGetOwnerID(owner), tryGetRepositoryID(repo), form.Title, form.Content)
	if err != nil {
		log.Error("InsertEncryptedSecret: %v", err)
		ctx.Flash.Error(ctx.Tr("secrets.creation.failed"))
	} else {
		audit.Record(auditActionSwitch(owner, repo, audit.UserSecretAdd, audit.OrganizationSecretAdd, audit.RepositorySecretAdd), doer, auditScopeSwitch(owner, repo), s, "Added secret %s.", s.Name)

		ctx.Flash.Success(ctx.Tr("secrets.creation.success", s.Name))
	}

	ctx.Redirect(redirectURL)
}

func PerformSecretsDelete(ctx *context.Context, doer, owner *user_model.User, repo *repo_model.Repository, redirectURL string) {
	defer ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": redirectURL,
	})

	id := ctx.FormInt64("id")

	s := &secret_model.Secret{}
	if has, err := db.GetByID(ctx, id, s); err != nil {
		log.Error("GetByID failed: %v", err)
		ctx.Flash.Error(ctx.Tr("secrets.deletion.failed"))
		return
	} else if !has || s.OwnerID != tryGetOwnerID(owner) || s.RepoID != tryGetRepositoryID(repo) {
		ctx.Flash.Error(ctx.Tr("secrets.deletion.failed"))
		return
	}

	if _, err := db.DeleteByBean(ctx, &secret_model.Secret{ID: id}); err != nil {
		log.Error("Delete secret %d failed: %v", id, err)
		ctx.Flash.Error(ctx.Tr("secrets.deletion.failed"))
		return
	}

	audit.Record(auditActionSwitch(owner, repo, audit.UserSecretRemove, audit.OrganizationSecretRemove, audit.RepositorySecretRemove), doer, auditScopeSwitch(owner, repo), s, "Removed secret %s.", s.Name)

	ctx.Flash.Success(ctx.Tr("secrets.deletion.success"))
}

func tryGetOwnerID(owner *user_model.User) int64 {
	if owner == nil {
		return 0
	}
	return owner.ID
}

func tryGetRepositoryID(repo *repo_model.Repository) int64 {
	if repo == nil {
		return 0
	}
	return repo.ID
}

func auditActionSwitch(owner *user_model.User, repo *repo_model.Repository, userAction, orgAction, repoAction audit.Action) audit.Action {
	if owner == nil {
		return repoAction
	}
	if owner.IsOrganization() {
		return orgAction
	}
	return userAction
}

func auditScopeSwitch(owner *user_model.User, repo *repo_model.Repository) any {
	if owner != nil {
		return owner
	}
	return repo
}

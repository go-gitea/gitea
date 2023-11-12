// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/audit"
)

func CreateOrUpdateSecret(ctx context.Context, doer, owner *user_model.User, repo *repo_model.Repository, name, data string) (*secret_model.Secret, bool, error) {
	if err := ValidateName(name); err != nil {
		return nil, false, err
	}

	ss, err := secret_model.FindSecrets(ctx, secret_model.FindSecretsOptions{
		OwnerID: tryGetOwnerID(owner),
		RepoID:  tryGetRepositoryID(repo),
		Name:    name,
	})
	if err != nil {
		return nil, false, err
	}

	if len(ss) == 0 {
		s, err := secret_model.InsertEncryptedSecret(ctx, tryGetOwnerID(owner), tryGetRepositoryID(repo), name, data)
		if err != nil {
			return nil, false, err
		}

		audit.Record(ctx,
			auditActionSwitch(owner, repo, audit.UserSecretAdd, audit.OrganizationSecretAdd, audit.RepositorySecretAdd),
			doer,
			auditScopeSwitch(owner, repo),
			s,
			"Added secret %s.",
			s.Name,
		)

		return s, true, nil
	}

	s := ss[0]

	if err := secret_model.UpdateSecret(ctx, s.ID, data); err != nil {
		return nil, false, err
	}

	audit.Record(ctx,
		auditActionSwitch(owner, repo, audit.UserSecretUpdate, audit.OrganizationSecretUpdate, audit.RepositorySecretUpdate),
		doer,
		auditScopeSwitch(owner, repo),
		s,
		"Added secret %s.",
		s.Name,
	)

	return s, false, nil
}

func DeleteSecretByID(ctx context.Context, doer, owner *user_model.User, repo *repo_model.Repository, secretID int64) error {
	s, err := secret_model.FindSecrets(ctx, secret_model.FindSecretsOptions{
		OwnerID:  tryGetOwnerID(owner),
		RepoID:   tryGetRepositoryID(repo),
		SecretID: secretID,
	})
	if err != nil {
		return err
	}
	if len(s) != 1 {
		return secret_model.ErrSecretNotFound{}
	}

	return deleteSecret(ctx, doer, owner, repo, s[0])
}

func DeleteSecretByName(ctx context.Context, doer, owner *user_model.User, repo *repo_model.Repository, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	s, err := secret_model.FindSecrets(ctx, secret_model.FindSecretsOptions{
		OwnerID: tryGetOwnerID(owner),
		RepoID:  tryGetRepositoryID(repo),
		Name:    name,
	})
	if err != nil {
		return err
	}
	if len(s) != 1 {
		return secret_model.ErrSecretNotFound{}
	}

	return deleteSecret(ctx, doer, owner, repo, s[0])
}

func deleteSecret(ctx context.Context, doer, owner *user_model.User, repo *repo_model.Repository, s *secret_model.Secret) error {
	if _, err := db.DeleteByID(ctx, s.ID, s); err != nil {
		return err
	}

	audit.Record(ctx,
		auditActionSwitch(owner, repo, audit.UserSecretRemove, audit.OrganizationSecretRemove, audit.RepositorySecretRemove),
		doer,
		auditScopeSwitch(owner, repo),
		s,
		"Removed secret %s.",
		s.Name,
	)

	return nil
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

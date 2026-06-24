// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"context"
	"fmt"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	secret_model "gitea.dev/models/secret"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/audit"
)

// recordSecretAudit emits a secret audit event scoped to its repository,
// organization or user owner. verb/prep build the message ("Added ... for",
// "Removed ... of").
func recordSecretAudit(ctx context.Context, doer, owner *user_model.User, repo *repo_model.Repository, secret *secret_model.Secret, actions audit.ScopedActions, verb, prep string) {
	audit.RecordScoped(ctx, doer, owner, repo, actions, func(scope string) string {
		return fmt.Sprintf("%s secret %s %s %s.", verb, secret.Name, prep, scope)
	}, "secret", secret.Name)
}

func CreateOrUpdateSecret(ctx context.Context, doer, owner *user_model.User, repo *repo_model.Repository, name, data, description string) (*secret_model.Secret, bool, error) {
	if err := ValidateName(name); err != nil {
		return nil, false, err
	}

	ss, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
		OwnerID: tryGetOwnerID(owner),
		RepoID:  tryGetRepositoryID(repo),
		Name:    name,
	})
	if err != nil {
		return nil, false, err
	}

	if len(ss) == 0 {
		s, err := secret_model.InsertEncryptedSecret(ctx, tryGetOwnerID(owner), tryGetRepositoryID(repo), name, data, description)
		if err != nil {
			return nil, false, err
		}

		recordSecretAudit(ctx, doer, owner, repo, s, audit.ScopedActions{
			Repo: audit_model.RepositorySecretAdd,
			Org:  audit_model.OrganizationSecretAdd,
			User: audit_model.UserSecretAdd,
		}, "Added", "for")

		return s, true, nil
	}

	s := ss[0]

	if err := secret_model.UpdateSecret(ctx, s.ID, data, description); err != nil {
		return nil, false, err
	}

	recordSecretAudit(ctx, doer, owner, repo, s, audit.ScopedActions{
		Repo: audit_model.RepositorySecretUpdate,
		Org:  audit_model.OrganizationSecretUpdate,
		User: audit_model.UserSecretUpdate,
	}, "Updated", "of")

	return s, false, nil
}

func DeleteSecretByID(ctx context.Context, doer, owner *user_model.User, repo *repo_model.Repository, secretID int64) error {
	s, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
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

	s, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
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
	if _, err := db.DeleteByID[secret_model.Secret](ctx, s.ID); err != nil {
		return err
	}

	recordSecretAudit(ctx, doer, owner, repo, s, audit.ScopedActions{
		Repo: audit_model.RepositorySecretRemove,
		Org:  audit_model.OrganizationSecretRemove,
		User: audit_model.UserSecretRemove,
	}, "Removed", "of")

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

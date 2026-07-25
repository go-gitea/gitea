// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"context"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	secret_model "gitea.dev/models/secret"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/audit"
)

func CreateOrUpdateSecret(ctx context.Context, owner *user_model.User, repo *repo_model.Repository, name, data, description string) (*secret_model.Secret, bool, error) {
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

		audit.RecordScoped(ctx, owner, repo, audit.ScopedActions{
			Repo: audit_model.RepositorySecretAdd,
			Org:  audit_model.OrganizationSecretAdd,
			User: audit_model.UserSecretAdd,
		}, "secret", s.Name)

		return s, true, nil
	}

	s := ss[0]

	if err := secret_model.UpdateSecret(ctx, s.ID, data, description); err != nil {
		return nil, false, err
	}

	audit.RecordScoped(ctx, owner, repo, audit.ScopedActions{
		Repo: audit_model.RepositorySecretUpdate,
		Org:  audit_model.OrganizationSecretUpdate,
		User: audit_model.UserSecretUpdate,
	}, "secret", s.Name)

	return s, false, nil
}

func DeleteSecretByID(ctx context.Context, owner *user_model.User, repo *repo_model.Repository, secretID int64) error {
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

	return deleteSecret(ctx, owner, repo, s[0])
}

func DeleteSecretByName(ctx context.Context, owner *user_model.User, repo *repo_model.Repository, name string) error {
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

	return deleteSecret(ctx, owner, repo, s[0])
}

func deleteSecret(ctx context.Context, owner *user_model.User, repo *repo_model.Repository, s *secret_model.Secret) error {
	if _, err := db.DeleteByID[secret_model.Secret](ctx, s.ID); err != nil {
		return err
	}

	audit.RecordScoped(ctx, owner, repo, audit.ScopedActions{
		Repo: audit_model.RepositorySecretRemove,
		Org:  audit_model.OrganizationSecretRemove,
		User: audit_model.UserSecretRemove,
	}, "secret", s.Name)

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

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
func recordSecretAudit(ctx context.Context, doer, owner *user_model.User, repo *repo_model.Repository, secret *secret_model.Secret, repoAction, orgAction, userAction audit_model.Action, verb, prep string) {
	switch {
	case owner == nil:
		audit.Record(ctx, repoAction, doer, repo,
			fmt.Sprintf("%s secret %s %s repository %s.", verb, secret.Name, prep, repo.FullName()), "secret", secret.Name)
	case owner.IsOrganization():
		audit.Record(ctx, orgAction, doer, owner,
			fmt.Sprintf("%s secret %s %s organization %s.", verb, secret.Name, prep, owner.Name), "secret", secret.Name)
	default:
		audit.Record(ctx, userAction, doer, owner,
			fmt.Sprintf("%s secret %s %s user %s.", verb, secret.Name, prep, owner.Name), "secret", secret.Name)
	}
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

		recordSecretAudit(ctx, doer, owner, repo, s, audit_model.RepositorySecretAdd, audit_model.OrganizationSecretAdd, audit_model.UserSecretAdd, "Added", "for")

		return s, true, nil
	}

	s := ss[0]

	if err := secret_model.UpdateSecret(ctx, s.ID, data, description); err != nil {
		return nil, false, err
	}

	recordSecretAudit(ctx, doer, owner, repo, s, audit_model.RepositorySecretUpdate, audit_model.OrganizationSecretUpdate, audit_model.UserSecretUpdate, "Updated", "of")

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

	recordSecretAudit(ctx, doer, owner, repo, s, audit_model.RepositorySecretRemove, audit_model.OrganizationSecretRemove, audit_model.UserSecretRemove, "Removed", "of")

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

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"context"

	"code.gitea.io/gitea/models/db"
	secret_model "code.gitea.io/gitea/models/secret"
)

func CreateOrUpdateSecret(ctx context.Context, ownerID, repoID int64, name, data, description string) (*secret_model.Secret, bool, error) {
	if err := ValidateName(name); err != nil {
		return nil, false, err
	}

	s, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
		OwnerID: ownerID,
		RepoID:  repoID,
		Name:    name,
	})
	if err != nil {
		return nil, false, err
	}

	if len(s) == 0 {
		s, err := secret_model.InsertEncryptedSecret(ctx, ownerID, repoID, name, data, description)
		if err != nil {
			return nil, false, err
		}
		return s, true, nil
	}

	if err := secret_model.UpdateSecret(ctx, s[0].ID, data, description); err != nil {
		return nil, false, err
	}

	return s[0], false, nil
}

func DeleteSecretByID(ctx context.Context, ownerID, repoID, secretID int64) error {
	s, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
		OwnerID:  ownerID,
		RepoID:   repoID,
		SecretID: secretID,
	})
	if err != nil {
		return err
	}
	if len(s) != 1 {
		return secret_model.ErrSecretNotFound{}
	}

	return deleteSecret(ctx, s[0])
}

func DeleteSecretByName(ctx context.Context, ownerID, repoID int64, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	s, err := db.Find[secret_model.Secret](ctx, secret_model.FindSecretsOptions{
		OwnerID: ownerID,
		RepoID:  repoID,
		Name:    name,
	})
	if err != nil {
		return err
	}
	if len(s) != 1 {
		return secret_model.ErrSecretNotFound{}
	}

	return deleteSecret(ctx, s[0])
}

func deleteSecret(ctx context.Context, s *secret_model.Secret) error {
	if _, err := db.DeleteByID[secret_model.Secret](ctx, s.ID); err != nil {
		return err
	}
	return nil
}

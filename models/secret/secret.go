// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secret

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	secret_module "code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// Secret represents a secret
type Secret struct {
	ID          int64
	OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT"` // encrypted data
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
}

// ErrSecretNotFound represents a "secret not found" error.
type ErrSecretNotFound struct {
	Name string
}

// IsErrSecretNotFound checks if an error is a ErrSecretNotFound.
func IsErrSecretNotFound(err error) bool {
	_, ok := err.(ErrSecretNotFound)
	return ok
}

func (err ErrSecretNotFound) Error() string {
	return fmt.Sprintf("secret was not found [name: %s]", err.Name)
}

func (err ErrSecretNotFound) Unwrap() error {
	return util.ErrNotExist
}

// newSecret Creates a new already encrypted secret
func newSecret(ownerID, repoID int64, name, data string) *Secret {
	return &Secret{
		OwnerID: ownerID,
		RepoID:  repoID,
		Name:    strings.ToUpper(name),
		Data:    data,
	}
}

// InsertEncryptedSecret Creates, encrypts, and validates a new secret with yet unencrypted data and insert into database
func InsertEncryptedSecret(ctx context.Context, ownerID, repoID int64, name, data string) (*Secret, error) {
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, data)
	if err != nil {
		return nil, err
	}
	secret := newSecret(ownerID, repoID, name, encrypted)
	if err := secret.Validate(); err != nil {
		return secret, err
	}
	return secret, db.Insert(ctx, secret)
}

func init() {
	db.RegisterModel(new(Secret))
}

func (s *Secret) Validate() error {
	if s.OwnerID == 0 && s.RepoID == 0 {
		return errors.New("the secret is not bound to any scope")
	}
	return nil
}

type FindSecretsOptions struct {
	db.ListOptions
	OwnerID int64
	RepoID  int64
}

func (opts *FindSecretsOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	return cond
}

func FindSecrets(ctx context.Context, opts FindSecretsOptions) ([]*Secret, error) {
	var secrets []*Secret
	sess := db.GetEngine(ctx)
	if opts.PageSize != 0 {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}
	return secrets, sess.
		Where(opts.toConds()).
		Find(&secrets)
}

// CountSecrets counts the secrets
func CountSecrets(ctx context.Context, opts *FindSecretsOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toConds()).Count(new(Secret))
}

// UpdateSecret changes org or user reop secret.
func UpdateSecret(ctx context.Context, orgID, repoID int64, name, data string) error {
	sc := new(Secret)
	name = strings.ToUpper(name)
	has, err := db.GetEngine(ctx).
		Where("owner_id=?", orgID).
		And("repo_id=?", repoID).
		And("name=?", name).
		Get(sc)
	if err != nil {
		return err
	} else if !has {
		return ErrSecretNotFound{Name: name}
	}

	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, data)
	if err != nil {
		return err
	}

	sc.Data = encrypted
	_, err = db.GetEngine(ctx).ID(sc.ID).Cols("data").Update(sc)
	return err
}

// DeleteSecret deletes secret from an organization.
func DeleteSecret(ctx context.Context, orgID, repoID int64, name string) error {
	sc := new(Secret)
	has, err := db.GetEngine(ctx).
		Where("owner_id=?", orgID).
		And("repo_id=?", repoID).
		And("name=?", strings.ToUpper(name)).
		Get(sc)
	if err != nil {
		return err
	} else if !has {
		return ErrSecretNotFound{Name: name}
	}

	if _, err := db.GetEngine(ctx).ID(sc.ID).Delete(new(Secret)); err != nil {
		return fmt.Errorf("Delete: %w", err)
	}

	return nil
}

// CreateOrUpdateSecret creates or updates a secret and returns true if it was created
func CreateOrUpdateSecret(ctx context.Context, orgID, repoID int64, name, data string) (bool, error) {
	sc := new(Secret)
	name = strings.ToUpper(name)
	has, err := db.GetEngine(ctx).
		Where("owner_id=?", orgID).
		And("repo_id=?", repoID).
		And("name=?", name).
		Get(sc)
	if err != nil {
		return false, err
	}

	if !has {
		_, err = InsertEncryptedSecret(ctx, orgID, repoID, name, data)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	if err := UpdateSecret(ctx, orgID, repoID, name, data); err != nil {
		return false, err
	}

	return false, nil
}

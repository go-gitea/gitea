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

func (err ErrSecretNotFound) Error() string {
	return fmt.Sprintf("secret was not found [name: %s]", err.Name)
}

func (err ErrSecretNotFound) Unwrap() error {
	return util.ErrNotExist
}

// InsertEncryptedSecret Creates, encrypts, and validates a new secret with yet unencrypted data and insert into database
func InsertEncryptedSecret(ctx context.Context, ownerID, repoID int64, name, data string) (*Secret, error) {
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, data)
	if err != nil {
		return nil, err
	}
	secret := &Secret{
		OwnerID: ownerID,
		RepoID:  repoID,
		Name:    strings.ToUpper(name),
		Data:    encrypted,
	}
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
	OwnerID  int64
	RepoID   int64
	SecretID int64
	Name     string
}

func (opts *FindSecretsOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.SecretID != 0 {
		cond = cond.And(builder.Eq{"id": opts.SecretID})
	}
	if opts.Name != "" {
		cond = cond.And(builder.Eq{"name": strings.ToUpper(opts.Name)})
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
func UpdateSecret(ctx context.Context, secretID int64, data string) error {
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, data)
	if err != nil {
		return err
	}

	s := &Secret{
		Data: encrypted,
	}
	affected, err := db.GetEngine(ctx).ID(secretID).Cols("data").Update(s)
	if affected != 1 {
		return ErrSecretNotFound{}
	}
	return err
}

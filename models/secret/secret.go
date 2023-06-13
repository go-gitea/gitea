// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secret

import (
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/models/db"
	secret_module "code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

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

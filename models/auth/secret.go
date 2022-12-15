// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"regexp"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type ErrSecretInvalidValue struct {
	Name *string
	Data *string
}

func (err ErrSecretInvalidValue) Error() string {
	if err.Name != nil {
		return fmt.Sprintf("secret name %q is invalid", *err.Name)
	}
	if err.Data != nil {
		return fmt.Sprintf("secret data %q is invalid", *err.Data)
	}
	return util.ErrInvalidArgument.Error()
}

func (err ErrSecretInvalidValue) Unwrap() error {
	return util.ErrInvalidArgument
}

var nameRE = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9-_.]*$")

// Secret represents a secret
type Secret struct {
	ID          int64
	OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOTNULL"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOTNULL"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOTNULL"`
	Data        string             `xorm:"LONGTEXT"` // encrypted data, or plaintext data if there's no master key
	CreatedUnix timeutil.TimeStamp `xorm:"created NOTNULL"`
}

func init() {
	db.RegisterModel(new(Secret))
}

// Validate validates the required fields and formats.
func (s *Secret) Validate() error {
	switch {
	case len(s.Name) == 0:
		return ErrSecretInvalidValue{Name: &s.Name}
	case len(s.Data) == 0:
		return ErrSecretInvalidValue{Data: &s.Data}
	case nameRE.MatchString(s.Name):
		return ErrSecretInvalidValue{Name: &s.Name}
	default:
		return nil
	}
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

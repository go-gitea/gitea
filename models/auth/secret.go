// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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

// Secret represents a secret
type Secret struct {
	ID          int64
	OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT"` // encrypted data
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func init() {
	db.RegisterModel(new(Secret))
}

var (
	secretNameReg = regexp.MustCompile("^[A-Z_][A-Z0-9_]*$")
	forbiddenSecretPrefixReg = regexp.MustCompile("^GIT(EA|HUB)_")
	)

// Validate validates the required fields and formats.
func (s *Secret) Validate() error {
	switch {
	case len(s.Name) == 0:
		return ErrSecretInvalidValue{Name: &s.Name}
	case len(s.Data) == 0:
		return ErrSecretInvalidValue{Data: &s.Data}
	case !secretNameReg.MatchString(s.Name) ||
		forbiddenSecretPrefixReg.MatchSting(s.Name):
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

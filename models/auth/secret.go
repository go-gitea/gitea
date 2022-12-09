// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"fmt"
	"regexp"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
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
	UserID      int64              `xorm:"index NOTNULL"`
	RepoID      int64              `xorm:"index NOTNULL"`
	Name        string             `xorm:"NOTNULL"`
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

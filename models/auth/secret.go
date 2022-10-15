// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"regexp"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

type ErrSecretNameInvalid struct {
	Name string
}

func (err ErrSecretNameInvalid) Error() string {
	return fmt.Sprintf("secret name %s is invalid", err.Name)
}

type ErrSecretDataInvalid struct {
	Data string
}

func (err ErrSecretDataInvalid) Error() string {
	return fmt.Sprintf("secret data %s is invalid", err.Data)
}

var nameRE = regexp.MustCompile("[^a-zA-Z0-9-_.]+")

// Secret represents a secret
type Secret struct {
	ID          int64
	UserID      int64              `xorm:"index NOTNULL"`
	RepoID      int64              `xorm:"index NOTNULL"`
	Name        string             `xorm:"NOTNULL"`
	Data        string             `xorm:"TEXT"`
	PullRequest bool               `xorm:"NOTNULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOTNULL"`
}

func init() {
	db.RegisterModel(new(Secret))
}

// Validate validates the required fields and formats.
func (s *Secret) Validate() error {
	switch {
	case len(s.Name) == 0:
		return ErrSecretNameInvalid{Name: s.Name}
	case len(s.Data) == 0:
		return ErrSecretDataInvalid{Data: s.Data}
	case nameRE.MatchString(s.Name):
		return ErrSecretNameInvalid{Name: s.Name}
	default:
		return nil
	}
}

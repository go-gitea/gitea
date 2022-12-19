// Copyright 2022 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package foreignreference

import (
	"fmt"

	"code.gitea.io/gitea/modules/util"
)

// ErrLocalIndexNotExist represents a "LocalIndexNotExist" kind of error.
type ErrLocalIndexNotExist struct {
	RepoID       int64
	ForeignIndex int64
	Type         string
}

// ErrLocalIndexNotExist checks if an error is a ErrLocalIndexNotExist.
func IsErrLocalIndexNotExist(err error) bool {
	_, ok := err.(ErrLocalIndexNotExist)
	return ok
}

func (err ErrLocalIndexNotExist) Error() string {
	return fmt.Sprintf("repository %d has no LocalIndex for ForeignIndex %d of type %s", err.RepoID, err.ForeignIndex, err.Type)
}

func (err ErrLocalIndexNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrForeignIndexNotExist represents a "ForeignIndexNotExist" kind of error.
type ErrForeignIndexNotExist struct {
	RepoID     int64
	LocalIndex int64
	Type       string
}

// ErrForeignIndexNotExist checks if an error is a ErrForeignIndexNotExist.
func IsErrForeignIndexNotExist(err error) bool {
	_, ok := err.(ErrForeignIndexNotExist)
	return ok
}

func (err ErrForeignIndexNotExist) Error() string {
	return fmt.Sprintf("repository %d has no ForeignIndex for LocalIndex %d of type %s", err.RepoID, err.LocalIndex, err.Type)
}

func (err ErrForeignIndexNotExist) Unwrap() error {
	return util.ErrNotExist
}

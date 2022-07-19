// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package foreignreference

import (
	"fmt"
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

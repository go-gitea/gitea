// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/modules/util"
)

type ErrGroupNotExist struct {
	ID int64
}

// IsErrGroupNotExist checks if an error is a ErrCommentNotExist.
func IsErrGroupNotExist(err error) bool {
	var errGroupNotExist ErrGroupNotExist
	ok := errors.As(err, &errGroupNotExist)
	return ok
}

func (err ErrGroupNotExist) Error() string {
	return fmt.Sprintf("group does not exist [id: %d]", err.ID)
}

func (err ErrGroupNotExist) Unwrap() error {
	return util.ErrNotExist
}

type ErrGroupTooDeep struct {
	ID int64
}

func IsErrGroupTooDeep(err error) bool {
	var errGroupTooDeep ErrGroupTooDeep
	ok := errors.As(err, &errGroupTooDeep)
	return ok
}

func (err ErrGroupTooDeep) Error() string {
	return fmt.Sprintf("group has reached or exceeded the subgroup nesting limit [id: %d]", err.ID)
}

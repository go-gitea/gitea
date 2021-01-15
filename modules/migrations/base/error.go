// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "errors"

var (
	// ErrRepoNotCreated returns the error that repository not created
	ErrRepoNotCreated = errors.New("repository is not created yet")
)

// ErrNotSupported represents status if a downloader do not supported something.
type ErrNotSupported struct {
}

// ErrNotSupported checks if an error is an ErrNotSupported
func IsErrNotSupported(err error) bool {
	_, ok := err.(ErrNotSupported)
	return ok
}

func (err ErrNotSupported) Error() string {
	return "not supported"
}

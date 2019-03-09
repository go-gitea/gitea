// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "errors"

var (
	// ErrNotImplemented returns the error not implemented
	ErrNotImplemented = errors.New("not implemented")
	// ErrNotSupported returns the error not supported
	ErrNotSupported = errors.New("not supported")
)

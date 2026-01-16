// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"errors"
	"fmt"
)

// ConcatenateError concatenates an error with stderr string
// FIXME: use RunStdError instead
func ConcatenateError(err error, stderr string) error {
	if len(stderr) == 0 {
		return err
	}
	return errors.Join(fmt.Errorf("%w - %s", err, stderr), &runStdError{err: err, stderr: stderr})
}

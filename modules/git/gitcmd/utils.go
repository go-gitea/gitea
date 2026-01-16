// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"fmt"

	"code.gitea.io/gitea/modules/util"
)

// ConcatenateError concatenates an error with stderr string
// FIXME: use RunStdError instead
func ConcatenateError(err error, stderr string) error {
	if len(stderr) == 0 {
		return err
	}
	errMsg := fmt.Sprintf("%s - %s", err.Error(), stderr)
	return util.ErrorWrap(&runStdError{err: err, stderr: stderr, errMsg: errMsg}, "%s", errMsg)
}

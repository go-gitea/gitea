// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"fmt"
	"io"

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

func safeClosePtrCloser[T *io.ReadCloser | *io.WriteCloser](c T) {
	switch v := any(c).(type) {
	case *io.ReadCloser:
		if v != nil && *v != nil {
			_ = (*v).Close()
		}
	case *io.WriteCloser:
		if v != nil && *v != nil {
			_ = (*v).Close()
		}
	default:
		panic("unsupported type")
	}
}

func safeAssignPipe[T any](p *T, fn func() (T, error)) (bool, error) {
	if p == nil {
		return false, nil
	}
	v, err := fn()
	if err != nil {
		return false, err
	}
	*p = v
	return true, nil
}

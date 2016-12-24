// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"time"
)

// ErrExecTimeout error when exec timed out
type ErrExecTimeout struct {
	Duration time.Duration
}

// IsErrExecTimeout if some error is ErrExecTimeout
func IsErrExecTimeout(err error) bool {
	_, ok := err.(ErrExecTimeout)
	return ok
}

func (err ErrExecTimeout) Error() string {
	return fmt.Sprintf("execution is timeout [duration: %v]", err.Duration)
}

// ErrNotExist commit not exist error
type ErrNotExist struct {
	ID      string
	RelPath string
}

// IsErrNotExist if some error is ErrNotExist
func IsErrNotExist(err error) bool {
	_, ok := err.(ErrNotExist)
	return ok
}

func (err ErrNotExist) Error() string {
	return fmt.Sprintf("object does not exist [id: %s, rel_path: %s]", err.ID, err.RelPath)
}

// ErrUnsupportedVersion error when required git version not matched
type ErrUnsupportedVersion struct {
	Required string
}

// IsErrUnsupportedVersion if some error is ErrUnsupportedVersion
func IsErrUnsupportedVersion(err error) bool {
	_, ok := err.(ErrUnsupportedVersion)
	return ok
}

func (err ErrUnsupportedVersion) Error() string {
	return fmt.Sprintf("Operation requires higher version [required: %s]", err.Required)
}

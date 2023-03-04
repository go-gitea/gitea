// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import "fmt"

// Error is a wrapped error describing the error results of Process Execution
type Error struct {
	PID         IDType
	Description string
	Err         error
	CtxErr      error
	Stdout      string
	Stderr      string
}

func (err *Error) Error() string {
	return fmt.Sprintf("exec(%s:%s) failed: %v(%v) stdout: %s stderr: %s", err.PID, err.Description, err.Err, err.CtxErr, err.Stdout, err.Stderr)
}

// Unwrap implements the unwrappable implicit interface for go1.13 Unwrap()
func (err *Error) Unwrap() error {
	return err.Err
}

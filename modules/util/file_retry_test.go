// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryWhenFileBusy(t *testing.T) {
	var callCount int
	testErrPath := &os.PathError{Op: "test", Path: "test", Err: syscall.EBUSY}
	fn := func() error {
		if callCount == 2 {
			return nil
		}
		callCount++
		return testErrPath
	}

	callCount = 0
	err := retryWhenFileBusyInternal(1, time.Millisecond, fn)
	assert.Equal(t, testErrPath, err)
	assert.Equal(t, 1, callCount)

	callCount = 0
	err = retryWhenFileBusyInternal(10, time.Millisecond, fn)
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)

	callCount = 0
	err = retryWhenFileBusyInternal(10, time.Millisecond, func() error {
		callCount++
		return io.ErrUnexpectedEOF
	})
	assert.Equal(t, io.ErrUnexpectedEOF, err)
	assert.Equal(t, 1, callCount)
}

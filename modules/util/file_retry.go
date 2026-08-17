// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// On Windows, when a file or directory is in use (opened), the file or directory is not able to be removed or renamed.
// When renaming or removing a local git repository directory:
// * the "cat-batch" git process might be running in a goroutine
// * there can be a data-race between the "cat-batch" git process cancel+exit and the repo rename
// So we need to retry the rename/remove operation for a few times when the "cat-batch" git process is exiting.
// ref: https://github.com/go-gitea/gitea/issues/16427, https://github.com/go-gitea/gitea/issues/16475, https://github.com/go-gitea/gitea/pull/16479
// Also some similar problems when removing a file, e.g.: https://github.com/go-gitea/gitea/issues/12339
//
// Usually, if no concurrent access to a file, use "os.Xxx", otherwise, use "util.XxxWithRetry"

func retryWhenFileBusyInternal(count int, delay time.Duration, f func() error) (err error) {
	const errWindowsSharingViolationError = syscall.Errno(32)
	for range count {
		err = f()
		if err == nil {
			break
		}
		isErrBusy := errors.Is(err, syscall.EBUSY) || errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE)
		isErrBusy = isErrBusy || (isOSWindows && errors.Is(err, errWindowsSharingViolationError))
		if !isErrBusy {
			break
		}
		time.Sleep(delay)
	}
	return err
}

func retryWhenFileBusy(f func() error) (err error) {
	return retryWhenFileBusyInternal(5, 100*time.Millisecond, f)
}

func RemoveWithRetry(path string) error {
	return retryWhenFileBusy(func() error {
		return os.Remove(path)
	})
}

func RemoveAllWithRetry(path string) error {
	return retryWhenFileBusy(func() error {
		return os.RemoveAll(path)
	})
}

func RenameWithRetry(oldpath, newpath string) error {
	return retryWhenFileBusy(func() error {
		return os.Rename(oldpath, newpath)
	})
}

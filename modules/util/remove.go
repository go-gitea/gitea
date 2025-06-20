// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"os"
	"runtime"
	"syscall"
	"time"
)

const windowsSharingViolationError syscall.Errno = 32

// Remove removes the named file or (empty) directory with at most 5 attempts.
func Remove(name string) error {
	var err error
	for range 5 {
		err = os.Remove(name)
		if err == nil {
			break
		}
		unwrapped := err.(*os.PathError).Err
		if unwrapped == syscall.EBUSY || unwrapped == syscall.ENOTEMPTY || unwrapped == syscall.EPERM || unwrapped == syscall.EMFILE || unwrapped == syscall.ENFILE {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if unwrapped == windowsSharingViolationError && runtime.GOOS == "windows" {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if unwrapped == syscall.ENOENT {
			// it's already gone
			return nil
		}
	}
	return err
}

// RemoveAll removes the named file or (empty) directory with at most 5 attempts.
func RemoveAll(name string) error {
	var err error
	for range 5 {
		err = os.RemoveAll(name)
		if err == nil {
			break
		}
		unwrapped := err.(*os.PathError).Err
		if unwrapped == syscall.EBUSY || unwrapped == syscall.ENOTEMPTY || unwrapped == syscall.EPERM || unwrapped == syscall.EMFILE || unwrapped == syscall.ENFILE {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if unwrapped == windowsSharingViolationError && runtime.GOOS == "windows" {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if unwrapped == syscall.ENOENT {
			// it's already gone
			return nil
		}
	}
	return err
}

// Rename renames (moves) oldpath to newpath with at most 5 attempts.
func Rename(oldpath, newpath string) error {
	var err error
	for i := range 5 {
		err = os.Rename(oldpath, newpath)
		if err == nil {
			break
		}
		unwrapped := err.(*os.LinkError).Err
		if unwrapped == syscall.EBUSY || unwrapped == syscall.ENOTEMPTY || unwrapped == syscall.EPERM || unwrapped == syscall.EMFILE || unwrapped == syscall.ENFILE {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if unwrapped == windowsSharingViolationError && runtime.GOOS == "windows" {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if i == 0 && os.IsNotExist(err) {
			return err
		}

		if unwrapped == syscall.ENOENT {
			// it's already gone
			return nil
		}
	}
	return err
}

// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"fmt"
)

type ErrLFS struct {
	OID string
	Err error
}

type ErrLFSDownload struct {
	ErrLFS
}

type ErrLFSUpload struct {
	ErrLFS
}

type ErrLFSVerify struct {
	ErrLFS
}

func (e *ErrLFS) Error() string {
	return fmt.Sprintf("LFS error for object %s: %v", e.OID, e.Err)
}

func (e *ErrLFS) Unwrap() error {
	return e.Err
}

func (e *ErrLFSDownload) Error() string {
	return fmt.Sprintf("LFS error while downloading [%s]: %s", e.OID, e.Err)
}

func (e *ErrLFSUpload) Error() string {
	return fmt.Sprintf("LFS error while uploading [%s]: %s", e.OID, e.Err)
}

func (e *ErrLFSVerify) Error() string {
	return fmt.Sprintf("LFS error while verifying [%s]: %s", e.OID, e.Err)
}

func (e *ErrLFSDownload) Is(target error) bool {
	_, ok := target.(*ErrLFSDownload)
	return ok
}

func (e *ErrLFSUpload) Is(target error) bool {
	_, ok := target.(*ErrLFSUpload)
	return ok
}

func (e *ErrLFSVerify) Is(target error) bool {
	_, ok := target.(*ErrLFSVerify)
	return ok
}

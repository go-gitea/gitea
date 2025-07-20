// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

// FileStatus represents the status of a file in the disk.
type FileStatus int

const (
	FileStatusNormal      FileStatus = iota + 1 // FileStatusNormal indicates the file is normal and exists on disk.
	FileStatusToBeDeleted                       // FileStatusToBeDeleted indicates the file is marked for deletion but still exists on disk.
)

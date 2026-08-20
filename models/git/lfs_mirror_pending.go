// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/timeutil"
)

// LFSMirrorPending tracks LFS pointers discovered during mirror sync whose
// content has not yet been successfully downloaded from upstream. This table is
// separate from lfs_meta_object so that rollback to a prior binary version does
// not leave phantom rows that the old code misinterprets as synced content.
type LFSMirrorPending struct {
	ID           int64              `xorm:"pk autoincr"`
	RepositoryID int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Oid          string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Size         int64              `xorm:"NOT NULL"`
	BlobSha      string             `xorm:"NOT NULL DEFAULT ''"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(LFSMirrorPending))
}

// AddLFSMirrorPending inserts a pending LFS pointer if it is not already
// tracked (neither in lfs_mirror_pending nor in lfs_meta_object).
// pointerBlob.Hash is the git object hash of the pointer file blob, used by gc_lfs to
// check reachability without reconstructing the blob content.
// Returns true if a new row was inserted.
func AddLFSMirrorPending(ctx context.Context, repoID int64, pointerBlob lfs.PointerBlob) (bool, error) {
	// Already fully synced in lfs_meta_object?
	has, err := db.GetEngine(ctx).Where("repository_id = ? AND oid = ?", repoID, pointerBlob.Oid).
		Exist(&LFSMetaObject{})
	if err != nil {
		return false, err
	}
	if has {
		return false, nil
	}

	// Already pending?
	has, err = db.GetEngine(ctx).Where("repository_id = ? AND oid = ?", repoID, pointerBlob.Oid).
		Exist(&LFSMirrorPending{})
	if err != nil {
		return false, err
	}
	if has {
		return false, nil
	}

	return true, db.Insert(ctx, &LFSMirrorPending{
		RepositoryID: repoID,
		Oid:          pointerBlob.Oid,
		Size:         pointerBlob.Size,
		BlobSha:      pointerBlob.Hash,
	})
}

// CountLFSMirrorPendingByRepoID returns the number of pending LFS objects for a repository.
func CountLFSMirrorPendingByRepoID(ctx context.Context, repoID int64) (int64, error) {
	return db.GetEngine(ctx).Where("repository_id = ?", repoID).Count(&LFSMirrorPending{})
}

// GetLFSMirrorPendingByRepoID returns all pending LFS objects for a repository.
func GetLFSMirrorPendingByRepoID(ctx context.Context, repoID int64) ([]*LFSMirrorPending, error) {
	var objects []*LFSMirrorPending
	err := db.GetEngine(ctx).Where("repository_id = ?", repoID).Find(&objects)
	return objects, err
}

// RemoveLFSMirrorPending deletes a pending entry after successful download.
func RemoveLFSMirrorPending(ctx context.Context, repoID int64, oid string) error {
	_, err := db.GetEngine(ctx).Where("repository_id = ? AND oid = ?", repoID, oid).
		Delete(&LFSMirrorPending{})
	return err
}

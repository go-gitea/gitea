// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"errors"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// ErrPackageBlobNotExist indicates a package blob not exist error
var ErrPackageBlobNotExist = errors.New("Package blob does not exist")

func init() {
	db.RegisterModel(new(PackageBlob))
}

// PackageBlob represents a package blob
type PackageBlob struct {
	ID          int64              `xorm:"pk autoincr"`
	Size        int64              `xorm:"NOT NULL DEFAULT 0"`
	HashMD5     string             `xorm:"hash_md5 char(32) UNIQUE(md5) INDEX NOT NULL"`
	HashSHA1    string             `xorm:"hash_sha1 char(40) UNIQUE(sha1) INDEX NOT NULL"`
	HashSHA256  string             `xorm:"hash_sha256 char(64) UNIQUE(sha256) INDEX NOT NULL"`
	HashSHA512  string             `xorm:"hash_sha512 char(128) UNIQUE(sha512) INDEX NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
}

// GetOrInsertBlob inserts a blob. If the blob exists already the existing blob is returned
func GetOrInsertBlob(ctx context.Context, pb *PackageBlob) (*PackageBlob, bool, error) {
	e := db.GetEngine(ctx)

	has, err := e.Get(pb)
	if err != nil {
		return nil, false, err
	}
	if has {
		return pb, true, nil
	}
	if _, err = e.Insert(pb); err != nil {
		return nil, false, err
	}
	return pb, false, nil
}

// GetBlobByID gets a blob by id
func GetBlobByID(ctx context.Context, blobID int64) (*PackageBlob, error) {
	pb := &PackageBlob{}

	has, err := db.GetEngine(ctx).ID(blobID).Get(pb)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageBlobNotExist
	}
	return pb, nil
}

// ExistPackageBlobWithSHA returns if a package blob exists with the provided sha
func ExistPackageBlobWithSHA(ctx context.Context, blobSha256 string) (bool, error) {
	return db.GetEngine(ctx).Exist(&PackageBlob{
		HashSHA256: blobSha256,
	})
}

// FindExpiredUnreferencedBlobs gets all blobs without associated files older than the specific duration
func FindExpiredUnreferencedBlobs(ctx context.Context, olderThan time.Duration) ([]*PackageBlob, error) {
	pbs := make([]*PackageBlob, 0, 10)
	return pbs, db.GetEngine(ctx).
		Table("package_blob").
		Join("LEFT", "package_file", "package_file.blob_id = package_blob.id").
		Where("package_file.id IS NULL AND package_blob.created_unix < ?", time.Now().Add(-olderThan).Unix()).
		Find(&pbs)
}

// DeleteBlobByID deletes a blob by id
func DeleteBlobByID(ctx context.Context, blobID int64) error {
	_, err := db.GetEngine(ctx).ID(blobID).Delete(&PackageBlob{})
	return err
}

// GetTotalBlobSize returns the total blobs size in bytes
func GetTotalBlobSize() (int64, error) {
	return db.GetEngine(db.DefaultContext).
		SumInt(&PackageBlob{}, "size")
}

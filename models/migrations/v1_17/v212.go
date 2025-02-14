// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddPackageTables(x *xorm.Engine) error {
	type Package struct {
		ID               int64  `xorm:"pk autoincr"`
		OwnerID          int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
		RepoID           int64  `xorm:"INDEX"`
		Type             string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name             string `xorm:"NOT NULL"`
		LowerName        string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		SemverCompatible bool   `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync(new(Package)); err != nil {
		return err
	}

	type PackageVersion struct {
		ID            int64              `xorm:"pk autoincr"`
		PackageID     int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CreatorID     int64              `xorm:"NOT NULL DEFAULT 0"`
		Version       string             `xorm:"NOT NULL"`
		LowerVersion  string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
		IsInternal    bool               `xorm:"INDEX NOT NULL DEFAULT false"`
		MetadataJSON  string             `xorm:"metadata_json TEXT"`
		DownloadCount int64              `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Sync(new(PackageVersion)); err != nil {
		return err
	}

	type PackageProperty struct {
		ID      int64  `xorm:"pk autoincr"`
		RefType int64  `xorm:"INDEX NOT NULL"`
		RefID   int64  `xorm:"INDEX NOT NULL"`
		Name    string `xorm:"INDEX NOT NULL"`
		Value   string `xorm:"TEXT NOT NULL"`
	}

	if err := x.Sync(new(PackageProperty)); err != nil {
		return err
	}

	type PackageFile struct {
		ID           int64              `xorm:"pk autoincr"`
		VersionID    int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
		BlobID       int64              `xorm:"INDEX NOT NULL"`
		Name         string             `xorm:"NOT NULL"`
		LowerName    string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CompositeKey string             `xorm:"UNIQUE(s) INDEX"`
		IsLead       bool               `xorm:"NOT NULL DEFAULT false"`
		CreatedUnix  timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
	}

	if err := x.Sync(new(PackageFile)); err != nil {
		return err
	}

	type PackageBlob struct {
		ID          int64              `xorm:"pk autoincr"`
		Size        int64              `xorm:"NOT NULL DEFAULT 0"`
		HashMD5     string             `xorm:"hash_md5 char(32) UNIQUE(md5) INDEX NOT NULL"`
		HashSHA1    string             `xorm:"hash_sha1 char(40) UNIQUE(sha1) INDEX NOT NULL"`
		HashSHA256  string             `xorm:"hash_sha256 char(64) UNIQUE(sha256) INDEX NOT NULL"`
		HashSHA512  string             `xorm:"hash_sha512 char(128) UNIQUE(sha512) INDEX NOT NULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
	}

	if err := x.Sync(new(PackageBlob)); err != nil {
		return err
	}

	type PackageBlobUpload struct {
		ID             string             `xorm:"pk"`
		BytesReceived  int64              `xorm:"NOT NULL DEFAULT 0"`
		HashStateBytes []byte             `xorm:"BLOB"`
		CreatedUnix    timeutil.TimeStamp `xorm:"created NOT NULL"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"updated INDEX NOT NULL"`
	}

	return x.Sync(new(PackageBlobUpload))
}

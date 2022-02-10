// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addPackageTables(x *xorm.Engine) error {
	type Package struct {
		ID               int64  `xorm:"pk autoincr"`
		OwnerID          int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
		RepoID           int64  `xorm:"INDEX"`
		Type             string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name             string
		LowerName        string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		SemverCompatible bool
	}

	if err := x.Sync2(new(Package)); err != nil {
		return err
	}

	type PackageVersion struct {
		ID            int64 `xorm:"pk autoincr"`
		PackageID     int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CreatorID     int64
		Version       string
		LowerVersion  string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CompositeKey  string             `xorm:"UNIQUE(s) INDEX"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
		MetadataJSON  string             `xorm:"TEXT metadata_json"`
		DownloadCount int64
	}

	if err := x.Sync2(new(PackageVersion)); err != nil {
		return err
	}

	type PackageProperty struct {
		ID      int64  `xorm:"pk autoincr"`
		RefType int64  `xorm:"INDEX NOT NULL"`
		RefID   int64  `xorm:"INDEX NOT NULL"`
		Name    string `xorm:"INDEX NOT NULL"`
		Value   string `xorm:"INDEX NOT NULL"`
	}

	if err := x.Sync2(new(PackageProperty)); err != nil {
		return err
	}

	type PackageFile struct {
		ID           int64 `xorm:"pk autoincr"`
		VersionID    int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		BlobID       int64 `xorm:"INDEX NOT NULL"`
		Name         string
		LowerName    string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CompositeKey string `xorm:"UNIQUE(s) INDEX"`
		IsLead       bool
	}

	if err := x.Sync2(new(PackageFile)); err != nil {
		return err
	}

	type PackageBlob struct {
		ID         int64 `xorm:"pk autoincr"`
		Size       int64
		HashMD5    string `xorm:"hash_md5 char(32) UNIQUE(md5) INDEX NOT NULL"`
		HashSHA1   string `xorm:"hash_sha1 char(40) UNIQUE(sha1) INDEX NOT NULL"`
		HashSHA256 string `xorm:"hash_sha256 char(64) UNIQUE(sha256) INDEX NOT NULL"`
		HashSHA512 string `xorm:"hash_sha512 char(128) UNIQUE(sha512) INDEX NOT NULL"`
	}

	return x.Sync2(new(PackageBlob))
}

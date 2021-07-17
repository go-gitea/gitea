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
		ID          int64 `xorm:"pk autoincr"`
		RepoID      int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		CreatorID   int64
		Type        int `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Name        string
		LowerName   string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Version     string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		MetadataRaw string `xorm:"TEXT"`

		CreatedUnix timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	if err := x.Sync2(new(Package)); err != nil {
		return err
	}

	type PackageFile struct {
		ID         int64 `xorm:"pk autoincr"`
		PackageID  int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Size       int64
		Name       string
		LowerName  string `xorm:"UNIQUE(s) INDEX NOT NULL"`
		HashMD5    string `xorm:"hash_md5"`
		HashSHA1   string `xorm:"hash_sha1"`
		HashSHA256 string `xorm:"hash_sha256"`
		HashSHA512 string `xorm:"hash_sha512"`

		CreatedUnix timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync2(new(PackageFile))
}

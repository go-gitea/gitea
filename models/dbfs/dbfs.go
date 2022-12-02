// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dbfs

import (
	"context"
	"os"

	"code.gitea.io/gitea/models/db"
)

type dbfsMeta struct {
	ID              int64  `xorm:"pk autoincr"`
	FullPath        string `xorm:"VARCHAR(500) UNIQUE NOT NULL"`
	BlockSize       int64  `xorm:"BIGINT NOT NULL"`
	FileSize        int64  `xorm:"BIGINT NOT NULL"`
	CreateTimestamp int64  `xorm:"BIGINT NOT NULL"`
	ModifyTimestamp int64  `xorm:"BIGINT NOT NULL"`
}

type dbfsData struct {
	ID         int64  `xorm:"pk autoincr"`
	Revision   int64  `xorm:"BIGINT NOT NULL"`
	MetaID     int64  `xorm:"BIGINT index(meta_offset) NOT NULL"`
	BlobOffset int64  `xorm:"BIGINT index(meta_offset) NOT NULL"`
	BlobSize   int64  `xorm:"BIGINT NOT NULL"`
	BlobData   []byte `xorm:"BLOB NOT NULL"`
}

func init() {
	db.RegisterModel(new(dbfsMeta))
	db.RegisterModel(new(dbfsData))
}

func OpenFile(ctx context.Context, name string, flag int) (File, error) {
	f, err := newDbFile(ctx, name)
	if err != nil {
		return nil, err
	}
	err = f.open(flag)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func Open(ctx context.Context, name string) (File, error) {
	return OpenFile(ctx, name, os.O_RDONLY)
}

func Create(ctx context.Context, name string) (File, error) {
	return OpenFile(ctx, name, os.O_RDWR|os.O_CREATE|os.O_TRUNC)
}

func Rename(ctx context.Context, oldPath, newPath string) error {
	f, err := newDbFile(ctx, oldPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.renameTo(newPath)
}

func Remove(ctx context.Context, name string) error {
	f, err := newDbFile(ctx, name)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.delete()
}

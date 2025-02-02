// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dbfs

import (
	"context"
	"io/fs"
	"os"
	"path"
	"time"

	"code.gitea.io/gitea/models/db"
)

/*
The reasons behind the DBFS (database-filesystem) package:
When a Gitea action is running, the Gitea action server should collect and store all the logs.

The requirements are:
* The running logs must be stored across the cluster if the Gitea servers are deployed as a cluster.
* The logs will be archived to Object Storage (S3/MinIO, etc.) after a period of time.
* The Gitea action UI should be able to render the running logs and the archived logs.

Some possible solutions for the running logs:
* [Not ideal] Using local temp file: it can not be shared across the cluster.
* [Not ideal] Using shared file in the filesystem of git repository: although at the moment, the Gitea cluster's
	git repositories must be stored in a shared filesystem, in the future, Gitea may need a dedicated Git Service Server
	to decouple the shared filesystem. Then the action logs will become a blocker.
* [Not ideal] Record the logs in a database table line by line: it has a couple of problems:
	- It's difficult to make multiple increasing sequence (log line number) for different databases.
	- The database table will have a lot of rows and be affected by the big-table performance problem.
	- It's difficult to load logs by using the same interface as other storages.
  - It's difficult to calculate the size of the logs.

The DBFS solution:
* It can be used in a cluster.
* It can share the same interface (Read/Write/Seek) as other storages.
* It's very friendly to database because it only needs to store much fewer rows than the log-line solution.
* In the future, when Gitea action needs to limit the log size (other CI/CD services also do so), it's easier to calculate the log file size.
* Even sometimes the UI needs to render the tailing lines, the tailing lines can be found be counting the "\n" from the end of the file by seek.
  The seeking and finding is not the fastest way, but it's still acceptable and won't affect the performance too much.
*/

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

var _ fs.FileInfo = (*dbfsMeta)(nil)

func (m *dbfsMeta) Name() string {
	return path.Base(m.FullPath)
}

func (m *dbfsMeta) Size() int64 {
	return m.FileSize
}

func (m *dbfsMeta) Mode() fs.FileMode {
	return os.ModePerm
}

func (m *dbfsMeta) ModTime() time.Time {
	return fileTimestampToTime(m.ModifyTimestamp)
}

func (m *dbfsMeta) IsDir() bool {
	return false
}

func (m *dbfsMeta) Sys() any {
	return nil
}

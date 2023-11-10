// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dbfs

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
)

var defaultFileBlockSize int64 = 32 * 1024

type File interface {
	io.ReadWriteCloser
	io.Seeker
	fs.File
}

type file struct {
	ctx       context.Context
	metaID    int64
	fullPath  string
	blockSize int64

	allowRead  bool
	allowWrite bool
	offset     int64
}

var _ File = (*file)(nil)

func (f *file) readAt(fileMeta *dbfsMeta, offset int64, p []byte) (n int, err error) {
	if offset >= fileMeta.FileSize {
		return 0, io.EOF
	}

	blobPos := int(offset % f.blockSize)
	blobOffset := offset - int64(blobPos)
	blobRemaining := int(f.blockSize) - blobPos
	needRead := len(p)
	if needRead > blobRemaining {
		needRead = blobRemaining
	}
	if blobOffset+int64(blobPos)+int64(needRead) > fileMeta.FileSize {
		needRead = int(fileMeta.FileSize - blobOffset - int64(blobPos))
	}
	if needRead <= 0 {
		return 0, io.EOF
	}
	var fileData dbfsData
	ok, err := db.GetEngine(f.ctx).Where("meta_id = ? AND blob_offset = ?", f.metaID, blobOffset).Get(&fileData)
	if err != nil {
		return 0, err
	}
	blobData := fileData.BlobData
	if !ok {
		blobData = nil
	}

	canCopy := len(blobData) - blobPos
	if canCopy <= 0 {
		canCopy = 0
	}
	realRead := needRead
	if realRead > canCopy {
		realRead = canCopy
	}
	if realRead > 0 {
		copy(p[:realRead], fileData.BlobData[blobPos:blobPos+realRead])
	}
	for i := realRead; i < needRead; i++ {
		p[i] = 0
	}
	return needRead, nil
}

func (f *file) Read(p []byte) (n int, err error) {
	if f.metaID == 0 || !f.allowRead {
		return 0, os.ErrInvalid
	}

	fileMeta, err := findFileMetaByID(f.ctx, f.metaID)
	if err != nil {
		return 0, err
	}
	n, err = f.readAt(fileMeta, f.offset, p)
	f.offset += int64(n)
	return n, err
}

func (f *file) Write(p []byte) (n int, err error) {
	if f.metaID == 0 || !f.allowWrite {
		return 0, os.ErrInvalid
	}

	fileMeta, err := findFileMetaByID(f.ctx, f.metaID)
	if err != nil {
		return 0, err
	}

	needUpdateSize := false
	written := 0
	for len(p) > 0 {
		blobPos := int(f.offset % f.blockSize)
		blobOffset := f.offset - int64(blobPos)
		blobRemaining := int(f.blockSize) - blobPos
		needWrite := len(p)
		if needWrite > blobRemaining {
			needWrite = blobRemaining
		}
		buf := make([]byte, f.blockSize)
		readBytes, err := f.readAt(fileMeta, blobOffset, buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return written, err
		}
		copy(buf[blobPos:blobPos+needWrite], p[:needWrite])
		if blobPos+needWrite > readBytes {
			buf = buf[:blobPos+needWrite]
		} else {
			buf = buf[:readBytes]
		}

		fileData := dbfsData{
			MetaID:     fileMeta.ID,
			BlobOffset: blobOffset,
			BlobData:   buf,
		}
		if res, err := db.GetEngine(f.ctx).Exec("UPDATE dbfs_data SET revision=revision+1, blob_data=? WHERE meta_id=? AND blob_offset=?", buf, fileMeta.ID, blobOffset); err != nil {
			return written, err
		} else if updated, err := res.RowsAffected(); err != nil {
			return written, err
		} else if updated == 0 {
			if _, err = db.GetEngine(f.ctx).Insert(&fileData); err != nil {
				return written, err
			}
		}
		written += needWrite
		f.offset += int64(needWrite)
		if f.offset > fileMeta.FileSize {
			fileMeta.FileSize = f.offset
			needUpdateSize = true
		}
		p = p[needWrite:]
	}

	fileMetaUpdate := dbfsMeta{
		ModifyTimestamp: timeToFileTimestamp(time.Now()),
	}
	if needUpdateSize {
		fileMetaUpdate.FileSize = f.offset
	}
	if _, err := db.GetEngine(f.ctx).ID(fileMeta.ID).Update(fileMetaUpdate); err != nil {
		return written, err
	}
	return written, nil
}

func (f *file) Seek(n int64, whence int) (int64, error) {
	if f.metaID == 0 {
		return 0, os.ErrInvalid
	}

	newOffset := f.offset
	switch whence {
	case io.SeekStart:
		newOffset = n
	case io.SeekCurrent:
		newOffset += n
	case io.SeekEnd:
		size, err := f.size()
		if err != nil {
			return f.offset, err
		}
		newOffset = size + n
	default:
		return f.offset, os.ErrInvalid
	}
	if newOffset < 0 {
		return f.offset, os.ErrInvalid
	}
	f.offset = newOffset
	return newOffset, nil
}

func (f *file) Close() error {
	return nil
}

func (f *file) Stat() (os.FileInfo, error) {
	if f.metaID == 0 {
		return nil, os.ErrInvalid
	}

	fileMeta, err := findFileMetaByID(f.ctx, f.metaID)
	if err != nil {
		return nil, err
	}
	return fileMeta, nil
}

func timeToFileTimestamp(t time.Time) int64 {
	return t.UnixMicro()
}

func fileTimestampToTime(timestamp int64) time.Time {
	return time.UnixMicro(timestamp)
}

func (f *file) loadMetaByPath() (*dbfsMeta, error) {
	var fileMeta dbfsMeta
	if ok, err := db.GetEngine(f.ctx).Where("full_path = ?", f.fullPath).Get(&fileMeta); err != nil {
		return nil, err
	} else if ok {
		f.metaID = fileMeta.ID
		f.blockSize = fileMeta.BlockSize
		return &fileMeta, nil
	}
	return nil, nil
}

func (f *file) open(flag int) (err error) {
	// see os.OpenFile for flag values
	if flag&os.O_WRONLY != 0 {
		f.allowWrite = true
	} else if flag&os.O_RDWR != 0 {
		f.allowRead = true
		f.allowWrite = true
	} else /* O_RDONLY */ {
		f.allowRead = true
	}

	if f.allowWrite {
		if flag&os.O_CREATE != 0 {
			if flag&os.O_EXCL != 0 {
				// file must not exist.
				if f.metaID != 0 {
					return os.ErrExist
				}
			} else {
				// create a new file if none exists.
				if f.metaID == 0 {
					if err = f.createEmpty(); err != nil {
						return err
					}
				}
			}
		}
		if flag&os.O_TRUNC != 0 {
			if err = f.truncate(); err != nil {
				return err
			}
		}
		if flag&os.O_APPEND != 0 {
			if _, err = f.Seek(0, io.SeekEnd); err != nil {
				return err
			}
		}
		return nil
	}

	// read only mode
	if f.metaID == 0 {
		return os.ErrNotExist
	}
	return nil
}

func (f *file) createEmpty() error {
	if f.metaID != 0 {
		return os.ErrExist
	}
	now := time.Now()
	_, err := db.GetEngine(f.ctx).Insert(&dbfsMeta{
		FullPath:        f.fullPath,
		BlockSize:       f.blockSize,
		CreateTimestamp: timeToFileTimestamp(now),
		ModifyTimestamp: timeToFileTimestamp(now),
	})
	if err != nil {
		return err
	}
	if _, err = f.loadMetaByPath(); err != nil {
		return err
	}
	return nil
}

func (f *file) truncate() error {
	if f.metaID == 0 {
		return os.ErrNotExist
	}
	return db.WithTx(f.ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Exec("UPDATE dbfs_meta SET file_size = 0 WHERE id = ?", f.metaID); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Delete(&dbfsData{MetaID: f.metaID}); err != nil {
			return err
		}
		return nil
	})
}

func (f *file) renameTo(newPath string) error {
	if f.metaID == 0 {
		return os.ErrNotExist
	}
	newPath = buildPath(newPath)
	return db.WithTx(f.ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Exec("UPDATE dbfs_meta SET full_path = ? WHERE id = ?", newPath, f.metaID); err != nil {
			return err
		}
		return nil
	})
}

func (f *file) delete() error {
	if f.metaID == 0 {
		return os.ErrNotExist
	}
	return db.WithTx(f.ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Delete(&dbfsMeta{ID: f.metaID}); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Delete(&dbfsData{MetaID: f.metaID}); err != nil {
			return err
		}
		return nil
	})
}

func (f *file) size() (int64, error) {
	if f.metaID == 0 {
		return 0, os.ErrNotExist
	}
	fileMeta, err := findFileMetaByID(f.ctx, f.metaID)
	if err != nil {
		return 0, err
	}
	return fileMeta.FileSize, nil
}

func findFileMetaByID(ctx context.Context, metaID int64) (*dbfsMeta, error) {
	var fileMeta dbfsMeta
	if ok, err := db.GetEngine(ctx).Where("id = ?", metaID).Get(&fileMeta); err != nil {
		return nil, err
	} else if ok {
		return &fileMeta, nil
	}
	return nil, nil
}

func buildPath(path string) string {
	path = filepath.Clean(path)
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "/")
	return strconv.Itoa(strings.Count(path, "/")) + ":" + path
}

func newDbFile(ctx context.Context, path string) (*file, error) {
	path = buildPath(path)
	f := &file{ctx: ctx, fullPath: path, blockSize: defaultFileBlockSize}
	if _, err := f.loadMetaByPath(); err != nil {
		return nil, err
	}
	return f, nil
}

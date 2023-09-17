// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	gouuid "github.com/google/uuid"
)

// ErrUploadNotExist represents a "UploadNotExist" kind of error.
type ErrUploadNotExist struct {
	ID   int64
	UUID string
}

// IsErrUploadNotExist checks if an error is a ErrUploadNotExist.
func IsErrUploadNotExist(err error) bool {
	_, ok := err.(ErrUploadNotExist)
	return ok
}

func (err ErrUploadNotExist) Error() string {
	return fmt.Sprintf("attachment does not exist [id: %d, uuid: %s]", err.ID, err.UUID)
}

func (err ErrUploadNotExist) Unwrap() error {
	return util.ErrNotExist
}

// Upload represent a uploaded file to a repo to be deleted when moved
type Upload struct {
	ID   int64  `xorm:"pk autoincr"`
	UUID string `xorm:"uuid UNIQUE"`
	Name string
}

func init() {
	db.RegisterModel(new(Upload))
}

// UploadLocalPath returns where uploads is stored in local file system based on given UUID.
func UploadLocalPath(uuid string) string {
	return path.Join(setting.Repository.Upload.TempPath, uuid[0:1], uuid[1:2], uuid)
}

// LocalPath returns where uploads are temporarily stored in local file system.
func (upload *Upload) LocalPath() string {
	return UploadLocalPath(upload.UUID)
}

// NewUpload creates a new upload object.
func NewUpload(ctx context.Context, name string, buf []byte, file multipart.File) (_ *Upload, err error) {
	upload := &Upload{
		UUID: gouuid.New().String(),
		Name: name,
	}

	localPath := upload.LocalPath()
	if err = os.MkdirAll(path.Dir(localPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("MkdirAll: %w", err)
	}

	fw, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("Create: %w", err)
	}
	defer fw.Close()

	if _, err = fw.Write(buf); err != nil {
		return nil, fmt.Errorf("Write: %w", err)
	} else if _, err = io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("Copy: %w", err)
	}

	if _, err := db.GetEngine(ctx).Insert(upload); err != nil {
		return nil, err
	}

	return upload, nil
}

// GetUploadByUUID returns the Upload by UUID
func GetUploadByUUID(ctx context.Context, uuid string) (*Upload, error) {
	upload := &Upload{}
	has, err := db.GetEngine(ctx).Where("uuid=?", uuid).Get(upload)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUploadNotExist{0, uuid}
	}
	return upload, nil
}

// GetUploadsByUUIDs returns multiple uploads by UUIDS
func GetUploadsByUUIDs(ctx context.Context, uuids []string) ([]*Upload, error) {
	if len(uuids) == 0 {
		return []*Upload{}, nil
	}

	// Silently drop invalid uuids.
	uploads := make([]*Upload, 0, len(uuids))
	return uploads, db.GetEngine(ctx).In("uuid", uuids).Find(&uploads)
}

// DeleteUploads deletes multiple uploads
func DeleteUploads(ctx context.Context, uploads ...*Upload) (err error) {
	if len(uploads) == 0 {
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	ids := make([]int64, len(uploads))
	for i := 0; i < len(uploads); i++ {
		ids[i] = uploads[i].ID
	}
	if _, err = db.GetEngine(ctx).
		In("id", ids).
		Delete(new(Upload)); err != nil {
		return fmt.Errorf("delete uploads: %w", err)
	}

	if err = committer.Commit(); err != nil {
		return err
	}

	for _, upload := range uploads {
		localPath := upload.LocalPath()
		isFile, err := util.IsFile(localPath)
		if err != nil {
			log.Error("Unable to check if %s is a file. Error: %v", localPath, err)
		}
		if !isFile {
			continue
		}

		if err := util.Remove(localPath); err != nil {
			return fmt.Errorf("remove upload: %w", err)
		}
	}

	return nil
}

// DeleteUploadByUUID deletes a upload by UUID
func DeleteUploadByUUID(ctx context.Context, uuid string) error {
	upload, err := GetUploadByUUID(ctx, uuid)
	if err != nil {
		if IsErrUploadNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetUploadByUUID: %w", err)
	}

	if err := DeleteUploads(ctx, upload); err != nil {
		return fmt.Errorf("DeleteUpload: %w", err)
	}

	return nil
}

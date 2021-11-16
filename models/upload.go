// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
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

//  ____ ___        .__                    .___ ___________.___.__
// |    |   \______ |  |   _________     __| _/ \_   _____/|   |  |   ____   ______
// |    |   /\____ \|  |  /  _ \__  \   / __ |   |    __)  |   |  | _/ __ \ /  ___/
// |    |  / |  |_> >  |_(  <_> ) __ \_/ /_/ |   |     \   |   |  |_\  ___/ \___ \
// |______/  |   __/|____/\____(____  /\____ |   \___  /   |___|____/\___  >____  >
//           |__|                   \/      \/       \/                  \/     \/
//

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
func NewUpload(name string, buf []byte, file multipart.File) (_ *Upload, err error) {
	upload := &Upload{
		UUID: gouuid.New().String(),
		Name: name,
	}

	localPath := upload.LocalPath()
	if err = os.MkdirAll(path.Dir(localPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("MkdirAll: %v", err)
	}

	fw, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("Create: %v", err)
	}
	defer fw.Close()

	if _, err = fw.Write(buf); err != nil {
		return nil, fmt.Errorf("Write: %v", err)
	} else if _, err = io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("Copy: %v", err)
	}

	if _, err := db.GetEngine(db.DefaultContext).Insert(upload); err != nil {
		return nil, err
	}

	return upload, nil
}

// GetUploadByUUID returns the Upload by UUID
func GetUploadByUUID(uuid string) (*Upload, error) {
	upload := &Upload{}
	has, err := db.GetEngine(db.DefaultContext).Where("uuid=?", uuid).Get(upload)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUploadNotExist{0, uuid}
	}
	return upload, nil
}

// GetUploadsByUUIDs returns multiple uploads by UUIDS
func GetUploadsByUUIDs(uuids []string) ([]*Upload, error) {
	if len(uuids) == 0 {
		return []*Upload{}, nil
	}

	// Silently drop invalid uuids.
	uploads := make([]*Upload, 0, len(uuids))
	return uploads, db.GetEngine(db.DefaultContext).In("uuid", uuids).Find(&uploads)
}

// DeleteUploads deletes multiple uploads
func DeleteUploads(uploads ...*Upload) (err error) {
	if len(uploads) == 0 {
		return nil
	}

	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	ids := make([]int64, len(uploads))
	for i := 0; i < len(uploads); i++ {
		ids[i] = uploads[i].ID
	}
	if _, err = sess.
		In("id", ids).
		Delete(new(Upload)); err != nil {
		return fmt.Errorf("delete uploads: %v", err)
	}

	if err = sess.Commit(); err != nil {
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
			return fmt.Errorf("remove upload: %v", err)
		}
	}

	return nil
}

// DeleteUploadByUUID deletes a upload by UUID
func DeleteUploadByUUID(uuid string) error {
	upload, err := GetUploadByUUID(uuid)
	if err != nil {
		if IsErrUploadNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetUploadByUUID: %v", err)
	}

	if err := DeleteUploads(upload); err != nil {
		return fmt.Errorf("DeleteUpload: %v", err)
	}

	return nil
}

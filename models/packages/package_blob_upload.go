// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	"errors"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// ErrPackageBlobUploadNotExist indicates a package blob upload not exist error
var ErrPackageBlobUploadNotExist = errors.New("Package blob upload does not exist")

func init() {
	db.RegisterModel(new(PackageBlobUpload))
}

// PackageBlobUpload represents a package blob upload
type PackageBlobUpload struct {
	ID             string             `xorm:"pk"`
	BytesReceived  int64              `xorm:"NOT NULL DEFAULT 0"`
	HashStateBytes []byte             `xorm:"BLOB"`
	CreatedUnix    timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"updated INDEX NOT NULL"`
}

// CreateBlobUpload inserts a blob upload
func CreateBlobUpload(ctx context.Context) (*PackageBlobUpload, error) {
	id, err := util.CryptoRandomString(25)
	if err != nil {
		return nil, err
	}

	pbu := &PackageBlobUpload{
		ID: strings.ToLower(id),
	}

	_, err = db.GetEngine(ctx).Insert(pbu)
	return pbu, err
}

// GetBlobUploadByID gets a blob upload by id
func GetBlobUploadByID(ctx context.Context, id string) (*PackageBlobUpload, error) {
	pbu := &PackageBlobUpload{}

	has, err := db.GetEngine(ctx).ID(id).Get(pbu)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageBlobUploadNotExist
	}
	return pbu, nil
}

// UpdateBlobUpload updates the blob upload
func UpdateBlobUpload(ctx context.Context, pbu *PackageBlobUpload) error {
	_, err := db.GetEngine(ctx).ID(pbu.ID).Update(pbu)
	return err
}

// DeleteBlobUploadByID deletes the blob upload
func DeleteBlobUploadByID(ctx context.Context, id string) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&PackageBlobUpload{})
	return err
}

// FindExpiredBlobUploads gets all expired blob uploads
func FindExpiredBlobUploads(ctx context.Context, olderThan time.Duration) ([]*PackageBlobUpload, error) {
	pbus := make([]*PackageBlobUpload, 0, 10)
	return pbus, db.GetEngine(ctx).
		Where("updated_unix < ?", time.Now().Add(-olderThan).Unix()).
		Find(&pbus)
}

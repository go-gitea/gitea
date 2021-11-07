// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/models/db"
)

func init() {
	db.RegisterModel(new(PackageFile))
}

var (
	// ErrDuplicatePackageFile indicates a duplicated package file error
	ErrDuplicatePackageFile = errors.New("Package file does exist already")
	// ErrPackageFileNotExist indicates a package file not exist error
	ErrPackageFileNotExist = errors.New("Package file does not exist")
)

// PackageFile represents a package file
type PackageFile struct {
	ID        int64 `xorm:"pk autoincr"`
	VersionID int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
	BlobID    int64 `xorm:"INDEX NOT NULL"`
	Name      string
	LowerName string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	IsLead    bool
}

// TryInsertFile inserts a file. If the file exists already ErrDuplicatePackageFile is returned
func TryInsertFile(ctx context.Context, pf *PackageFile) (*PackageFile, error) {
	e := db.GetEngine(ctx)

	key := &PackageFile{
		VersionID: pf.VersionID,
		LowerName: pf.LowerName,
	}

	has, err := e.Get(key)
	if err != nil {
		return nil, err
	}
	if has {
		return pf, ErrDuplicatePackageFile
	}
	if _, err = e.Insert(pf); err != nil {
		return nil, err
	}
	return pf, nil
}

// GetFilesByVersionID gets all files of a version
func GetFilesByVersionID(ctx context.Context, versionID int64) ([]*PackageFile, error) {
	pfs := make([]*PackageFile, 0, 10)
	return pfs, db.GetEngine(ctx).Where("version_id = ?", versionID).Find(&pfs)
}

// GetFileForVersionByName gets a file of a version by name
func GetFileForVersionByName(ctx context.Context, versionID int64, name string) (*PackageFile, error) {
	if name == "" {
		return nil, ErrPackageFileNotExist
	}

	pf := &PackageFile{
		VersionID: versionID,
		LowerName: strings.ToLower(name),
	}

	has, err := db.GetEngine(ctx).Get(pf)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageFileNotExist
	}
	return pf, nil
}

// DeleteFilesByVersionID deletes all files of a version
func DeleteFilesByVersionID(ctx context.Context, versionID int64) error {
	_, err := db.GetEngine(ctx).Where("version_id = ?", versionID).Delete(&PackageFile{})
	return err
}

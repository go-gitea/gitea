// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"strings"

	"code.gitea.io/gitea/modules/timeutil"
)

// PackageType specifies the different package types
type PackageType int

// Note: new type must append to the end of list to maintain compatibility.
const (
	PackageGeneric PackageType = iota
)

var (
	// ErrDuplicatePackage indicates a duplicated package error
	ErrDuplicatePackage = errors.New("Package does exist already")
	// ErrPackageNotExist indicates a package not exist error
	ErrPackageNotExist = errors.New("Package does not exist")
)

// Package represents a package
type Package struct {
	ID           int64 `xorm:"pk autoincr"`
	RepositoryID int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
	CreatorID    int64
	Type         PackageType `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name         string
	LowerName    string      `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Version      string      `xorm:"UNIQUE(s) INDEX NOT NULL"`
	MetaData     interface{} `xorm:"TEXT JSON"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// PackageFile represents files associated with a package
type PackageFile struct {
	ID        int64 `xorm:"pk autoincr"`
	PackageID int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Size      int64
	Name      string `xorm:"UNIQUE(s) NOT NULL"`
	LowerName string `xorm:"UNIQUE(s) INDEX NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// GetFiles loads all files associated with the package
func (p *Package) GetFiles() ([]*PackageFile, error) {
	packageFiles := make([]*PackageFile, 0, 10)
	return packageFiles, x.Where("package_id = ?", p.ID).Find(&packageFiles)
}

// TryInsertPackage inserts a package
// If a package already exists ErrDuplicatePackage is returned
func TryInsertPackage(p *Package) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	has, err := sess.Get(p)
	if err != nil {
		return err
	}

	if has {
		return ErrDuplicatePackage
	}

	if _, err = sess.Insert(p); err != nil {
		return err
	}

	return sess.Commit()
}

// DeletePackageByID deletes a package and its files by ID
func DeletePackageByID(packageID int64) error {
	if err := DeletePackageFilesByPackageID(packageID); err != nil {
		return err
	}

	_, err := x.ID(packageID).Delete(&Package{})
	return err
}

// DeletePackagesByRepositoryID deletes all packages of a repository
func DeletePackagesByRepositoryID(repositoryID int64) error {
	packages, err := GetPackagesByRepositoryID(repositoryID)
	if err != nil {
		return err
	}

	for _, p := range packages {
		if err := DeletePackageByID(p.ID); err != nil {
			return err
		}
	}

	return nil
}

// GetPackagesByRepositoryID returns all packages of a repository
func GetPackagesByRepositoryID(repositoryID int64) ([]*Package, error) {
	packages := make([]*Package, 0, 10)
	return packages, x.Where("repository_id = ?", repositoryID).Find(&packages)
}

// GetPackagesByName gets all repository packages with the specific name
func GetPackagesByName(repositoryID int64, packageType PackageType, packageName string) ([]*Package, error) {
	packages := make([]*Package, 0, 10)
	return packages, x.Where("repository_id = ? AND type = ? AND lower_name = ?", repositoryID, packageType, strings.ToLower(packageName)).Find(&packages)
}

// GetPackageByNameAndVersion gets a repository package by name and version
func GetPackageByNameAndVersion(repositoryID int64, packageType PackageType, packageName, packageVersion string) (*Package, error) {
	p := &Package{
		RepositoryID: repositoryID,
		Type:         packageType,
		LowerName:    strings.ToLower(packageName),
		Version:      packageVersion,
	}
	has, err := x.Get(p)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPackageNotExist
	}
	return p, nil
}

// InsertPackageFile inserts a package file
func InsertPackageFile(pf *PackageFile) error {
	_, err := x.Insert(pf)
	return err
}

// DeletePackageFileByID deletes a package file
func DeletePackageFileByID(fileID int64) error {
	_, err := x.ID(fileID).Delete(&PackageFile{})
	return err
}

// DeletePackageFilesByPackageID deletes all files associated with the package
func DeletePackageFilesByPackageID(packageID int64) error {
	_, err := x.Where("package_id = ?", packageID).Delete(&PackageFile{})
	return err
}

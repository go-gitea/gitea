// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"errors"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Package))
	db.RegisterModel(new(PackageFile))
}

// Type specifies the different package types
type Type int

// Note: new type must append to the end of list to maintain compatibility.
const (
	TypeGeneric  Type = iota
	TypeNuGet         // 1
	TypeNpm           // 2
	TypeMaven         // 3
	TypePyPI          // 4
	TypeRubyGems      // 5
)

func (pt Type) String() string {
	switch pt {
	case TypeGeneric:
		return "Generic"
	case TypeNuGet:
		return "NuGet"
	case TypeNpm:
		return "npm"
	case TypeMaven:
		return "Maven"
	case TypePyPI:
		return "PyPI"
	case TypeRubyGems:
		return "RubyGems"
	}
	return ""
}

// SVGName returns the name of the package type svg image
func (pt Type) SVGName() string {
	switch pt {
	case TypeGeneric:
		return "octicon-package"
	case TypeNuGet:
		return "gitea-nuget"
	case TypeNpm:
		return "gitea-npm"
	case TypeMaven:
		return "gitea-maven"
	case TypePyPI:
		return "gitea-python"
	case TypeRubyGems:
		return "gitea-rubygems"
	}
	return ""
}

var (
	// ErrDuplicatePackage indicates a duplicated package error
	ErrDuplicatePackage = errors.New("Package does exist already")
	// ErrPackageNotExist indicates a package not exist error
	ErrPackageNotExist = errors.New("Package does not exist")
	// ErrDuplicatePackageFile indicates a duplicated package file error
	ErrDuplicatePackageFile = errors.New("Package file does exist already")
	// ErrPackageFileNotExist indicates a package file not exist error
	ErrPackageFileNotExist = errors.New("Package file does not exist")
)

// Package represents a package
type Package struct {
	ID          int64 `xorm:"pk autoincr"`
	RepoID      int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
	CreatorID   int64
	Creator     *models.User `xorm:"-"`
	Type        Type         `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string
	LowerName   string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Version     string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	MetadataRaw string `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// LoadCreator loads the creator
func (p *Package) LoadCreator() error {
	if p.Creator == nil {
		var err error
		p.Creator, err = models.GetUserByID(p.CreatorID)
		return err
	}
	return nil
}

// PackageFile represents files associated with a package
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

// GetFiles loads all files associated with the package
func (p *Package) GetFiles() ([]*PackageFile, error) {
	packageFiles := make([]*PackageFile, 0, 10)
	return packageFiles, db.GetEngine(db.DefaultContext).Where("package_id = ?", p.ID).Find(&packageFiles)
}

// GetFileByName gets the specific package file by name
func (p *Package) GetFileByName(name string) (*PackageFile, error) {
	pf := &PackageFile{
		PackageID: p.ID,
		LowerName: strings.ToLower(name),
	}
	has, err := db.GetEngine(db.DefaultContext).Get(pf)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageFileNotExist
	}
	return pf, nil
}

// TryInsertPackage inserts a package
// If a package already exists ErrDuplicatePackage is returned
func TryInsertPackage(p *Package) (*Package, error) {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	key := &Package{
		RepoID:    p.RepoID,
		Type:      p.Type,
		LowerName: p.LowerName,
		Version:   p.Version,
	}

	has, err := sess.Get(key)
	if err != nil {
		return nil, err
	}
	if has {
		return key, ErrDuplicatePackage
	}
	if _, err = sess.Insert(p); err != nil {
		return nil, err
	}
	return p, sess.Commit()
}

// UpdatePackage updates a package
func UpdatePackage(p *Package) error {
	_, err := db.GetEngine(db.DefaultContext).ID(p.ID).Update(p)
	return err
}

// DeletePackageByID deletes a package and its files by ID
func DeletePackageByID(packageID int64) error {
	if err := DeletePackageFilesByPackageID(packageID); err != nil {
		return err
	}

	_, err := db.GetEngine(db.DefaultContext).ID(packageID).Delete(&Package{})
	return err
}

// DeletePackagesByRepositoryID deletes all packages of a repository
func DeletePackagesByRepositoryID(repositoryID int64) error {
	packages, err := GetPackagesByRepository(repositoryID)
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

// PackageSearchOptions are options for GetLatestPackagesGrouped
type PackageSearchOptions struct {
	RepoID int64
	Type   string
	Query  string
	db.Paginator
}

func (opts *PackageSearchOptions) toConds() builder.Cond {
	var cond builder.Cond = builder.Eq{"package.repo_id": opts.RepoID}

	switch opts.Type {
	case "generic":
		cond = cond.And(builder.Eq{"package.type": TypeGeneric})
	case "nuget":
		cond = cond.And(builder.Eq{"package.type": TypeNuGet})
	case "npm":
		cond = cond.And(builder.Eq{"package.type": TypeNpm})
	case "maven":
		cond = cond.And(builder.Eq{"package.type": TypeMaven})
	case "pypi":
		cond = cond.And(builder.Eq{"package.type": TypePyPI})
	case "rubygems":
		cond = cond.And(builder.Eq{"package.type": TypeRubyGems})
	}

	if opts.Query != "" {
		cond = cond.And(builder.Like{"package.lower_name", strings.ToLower(opts.Query)})
	}

	return cond
}

// GetPackages returns a list of all packages of the repository
func GetPackages(opts *PackageSearchOptions) ([]*Package, int64, error) {
	sess := db.GetEngine(db.DefaultContext).Where(opts.toConds())

	if opts.Paginator != nil {
		sess = db.SetSessionPagination(sess, opts)
	}

	packages := make([]*Package, 0, 10)
	count, err := sess.FindAndCount(&packages)
	return packages, count, err
}

// GetLatestPackagesGrouped returns a list of all packages in their latest version of the repository
func GetLatestPackagesGrouped(opts *PackageSearchOptions) ([]*Package, int64, error) {
	cond := opts.toConds().
		And(builder.Expr("p2.id IS NULL"))

	sess := db.GetEngine(db.DefaultContext).Where(cond).
		Table("package").
		Join("left", "package p2", "package.repo_id = p2.repo_id AND package.type = p2.type AND package.lower_name = p2.lower_name AND package.version < p2.version")

	if opts.Paginator != nil {
		sess = db.SetSessionPagination(sess, opts)
	}

	packages := make([]*Package, 0, 10)
	count, err := sess.FindAndCount(&packages)
	return packages, count, err
}

// GetPackageByID returns the package with the specific id
func GetPackageByID(packageID int64) (*Package, error) {
	p := &Package{}
	has, err := db.GetEngine(db.DefaultContext).ID(packageID).Get(p)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPackageNotExist
	}
	return p, nil
}

// HasRepositoryPackages tests if a repository has packages
func HasRepositoryPackages(repositoryID int64) (bool, error) {
	return db.GetEngine(db.DefaultContext).Where("repo_id = ?", repositoryID).Exist(&Package{})
}

// GetPackagesByRepository returns all packages of a repository
func GetPackagesByRepository(repositoryID int64) ([]*Package, error) {
	packages := make([]*Package, 0, 10)
	return packages, db.GetEngine(db.DefaultContext).Where("repo_id = ?", repositoryID).Find(&packages)
}

// GetPackagesByRepositoryAndType returns all packages of a repository with the specific type
func GetPackagesByRepositoryAndType(repositoryID int64, packageType Type) ([]*Package, error) {
	packages := make([]*Package, 0, 10)
	return packages, db.GetEngine(db.DefaultContext).Where("repo_id = ? AND type = ?", repositoryID, packageType).Find(&packages)
}

// GetPackagesByName gets all repository packages with the specific name
func GetPackagesByName(repositoryID int64, packageType Type, packageName string) ([]*Package, error) {
	packages := make([]*Package, 0, 10)
	return packages, db.GetEngine(db.DefaultContext).Where("repo_id = ? AND type = ? AND lower_name = ?", repositoryID, packageType, strings.ToLower(packageName)).Find(&packages)
}

// GetPackageByNameAndVersion gets a repository package by name and version
func GetPackageByNameAndVersion(repositoryID int64, packageType Type, packageName, packageVersion string) (*Package, error) {
	p := &Package{
		RepoID:    repositoryID,
		Type:      packageType,
		LowerName: strings.ToLower(packageName),
		Version:   packageVersion,
	}
	has, err := db.GetEngine(db.DefaultContext).Get(p)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrPackageNotExist
	}
	return p, nil
}

// GetPackagesByFilename gets a repository packages by filename
func GetPackagesByFilename(repositoryID int64, packageType Type, packageFilename string) ([]*Package, error) {
	var cond builder.Cond = builder.Eq{
		"package.repo_id":         repositoryID,
		"package.type":            packageType,
		"package_file.lower_name": strings.ToLower(packageFilename),
	}

	packages := make([]*Package, 0, 10)
	return packages, db.GetEngine(db.DefaultContext).
		Table("package").
		Where(cond).
		Join("INNER", "package_file", "package.id = package_file.package_id").
		Find(&packages)
}

// TryInsertPackageFile inserts a package file
func TryInsertPackageFile(pf *PackageFile) (*PackageFile, error) {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	key := &PackageFile{
		PackageID: pf.PackageID,
		LowerName: pf.LowerName,
	}

	has, err := sess.Get(key)
	if err != nil {
		return nil, err
	}
	if has {
		return key, ErrDuplicatePackageFile
	}
	if _, err = sess.Insert(pf); err != nil {
		return nil, err
	}
	return pf, sess.Commit()
}

// UpdatePackageFile updates a package file
func UpdatePackageFile(pf *PackageFile) error {
	_, err := db.GetEngine(db.DefaultContext).ID(pf.ID).Update(pf)
	return err
}

// DeletePackageFileByID deletes a package file
func DeletePackageFileByID(fileID int64) error {
	_, err := db.GetEngine(db.DefaultContext).ID(fileID).Delete(&PackageFile{})
	return err
}

// DeletePackageFilesByPackageID deletes all files associated with the package
func DeletePackageFilesByPackageID(packageID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Where("package_id = ?", packageID).Delete(&PackageFile{})
	return err
}

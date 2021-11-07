// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

var (
	// ErrDuplicatePackageVersion indicates a duplicated package version error
	ErrDuplicatePackageVersion = errors.New("Package version does exist already")
	// ErrPackageVersionNotExist indicates a package version not exist error
	ErrPackageVersionNotExist = errors.New("Package version does not exist")
)

func init() {
	db.RegisterModel(new(PackageVersion))
}

// PackageVersion represents a package version
type PackageVersion struct {
	ID            int64 `xorm:"pk autoincr"`
	PackageID     int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
	CreatorID     int64
	Version       string
	LowerVersion  string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	MetadataJSON  string             `xorm:"TEXT metadata_json"`
	DownloadCount int64
}

// GetOrInsertVersion inserts a version. If the same version exist already ErrDuplicatePackageVersion is returned
func GetOrInsertVersion(ctx context.Context, pv *PackageVersion) (*PackageVersion, error) {
	e := db.GetEngine(ctx)

	key := &PackageVersion{
		PackageID:    pv.PackageID,
		LowerVersion: pv.LowerVersion,
	}

	has, err := e.Get(key)
	if err != nil {
		return nil, err
	}
	if has {
		return key, ErrDuplicatePackageVersion
	}
	if _, err = e.Insert(pv); err != nil {
		return nil, err
	}
	return pv, nil
}

// UpdateVersion updates a version
func UpdateVersion(pv *PackageVersion) error {
	_, err := db.GetEngine(db.DefaultContext).ID(pv.ID).Update(pv)
	return err
}

// IncrementDownloadCounter increments the download counter of a version
func IncrementDownloadCounter(versionID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Exec("UPDATE `package_version` SET `download_count` = `download_count` + 1 WHERE `id` = ?", versionID)
	return err
}

// GetVersionByID gets a version by id
func GetVersionByID(ctx context.Context, versionID int64) (*PackageVersion, error) {
	pv := &PackageVersion{}

	has, err := db.GetEngine(ctx).ID(versionID).Get(pv)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageNotExist
	}
	return pv, nil
}

// GetVersionByNameAndVersion gets a version by name and version number
func GetVersionByNameAndVersion(ctx context.Context, repositoryID int64, packageType Type, name, version string) (*PackageVersion, error) {
	var cond builder.Cond = builder.Eq{
		"package.repo_id":    repositoryID,
		"package.type":       packageType,
		"package.lower_name": strings.ToLower(name),
	}
	pv := &PackageVersion{
		LowerVersion: strings.ToLower(version),
	}
	has, err := db.GetEngine(ctx).
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(cond).
		Get(pv)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageNotExist
	}

	return pv, nil
}

// GetVersionsByPackageType gets all versions of a specific type
func GetVersionsByPackageType(repositoryID int64, packageType Type) ([]*PackageVersion, error) {
	var cond builder.Cond = builder.Eq{
		"package.repo_id": repositoryID,
		"package.type":    packageType,
	}

	pvs := make([]*PackageVersion, 0, 10)
	return pvs, db.GetEngine(db.DefaultContext).
		Where(cond).
		Join("INNER", "package", "package.id = package_version.package_id").
		Find(&pvs)
}

// GetVersionsByPackageName gets all versions of a specific package
func GetVersionsByPackageName(repositoryID int64, packageType Type, name string) ([]*PackageVersion, error) {
	var cond builder.Cond = builder.Eq{
		"package.repo_id":    repositoryID,
		"package.type":       packageType,
		"package.lower_name": strings.ToLower(name),
	}

	pvs := make([]*PackageVersion, 0, 10)
	return pvs, db.GetEngine(db.DefaultContext).
		Where(cond).
		Join("INNER", "package", "package.id = package_version.package_id").
		Find(&pvs)
}

// GetVersionsByFilename gets all versions which are linked to a filename
func GetVersionsByFilename(repositoryID int64, packageType Type, filename string) ([]*PackageVersion, error) {
	var cond builder.Cond = builder.Eq{
		"package.repo_id":         repositoryID,
		"package.type":            packageType,
		"package_file.lower_name": strings.ToLower(filename),
	}

	pvs := make([]*PackageVersion, 0, 10)
	return pvs, db.GetEngine(db.DefaultContext).
		Where(cond).
		Join("INNER", "package_file", "package_file.version_id = package_version.id").
		Join("INNER", "package", "package.id = package_version.package_id").
		Find(&pvs)
}

// DeleteVersionByID deletes a version by id
func DeleteVersionByID(ctx context.Context, versionID int64) error {
	_, err := db.GetEngine(ctx).ID(versionID).Delete(&PackageVersion{})
	return err
}

// PackageSearchOptions are options for SearchXXX methods
type PackageSearchOptions struct {
	RepoID int64
	Type   string
	Query  string
	db.Paginator
}

func (opts *PackageSearchOptions) toConds() builder.Cond {
	var cond builder.Cond = builder.Eq{"package.repo_id": opts.RepoID}

	if opts.Type != "" {
		cond = cond.And(builder.Eq{"package.type": opts.Type})
	}

	if opts.Query != "" {
		cond = cond.And(builder.Like{"package.lower_name", strings.ToLower(opts.Query)})
	}

	return cond
}

// SearchVersions gets all versions of packages matching the search options
func SearchVersions(opts *PackageSearchOptions) ([]*PackageVersion, int64, error) {
	sess := db.GetEngine(db.DefaultContext).Where(opts.toConds()).
		Table("package_version").
		Join("INNER", "package", "package.id = package_version.package_id")

	if opts.Paginator != nil {
		sess = db.SetSessionPagination(sess, opts)
	}

	pvs := make([]*PackageVersion, 0, 10)
	count, err := sess.FindAndCount(&pvs)
	return pvs, count, err
}

// SearchLatestVersions gets the latest version of every package matching the search options
func SearchLatestVersions(opts *PackageSearchOptions) ([]*PackageVersion, int64, error) {
	cond := opts.toConds().
		And(builder.Expr("pv2.id IS NULL"))

	sess := db.GetEngine(db.DefaultContext).
		Table("package_version").
		Join("LEFT", "package_version pv2", "package_version.package_id = pv2.package_id AND package_version.version < pv2.version").
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(cond)

	if opts.Paginator != nil {
		sess = db.SetSessionPagination(sess, opts)
	}

	pvs := make([]*PackageVersion, 0, 10)
	count, err := sess.FindAndCount(&pvs)
	return pvs, count, err
}

// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(PackageFile))
}

var (
	// ErrDuplicatePackageFile indicates a duplicated package file error
	ErrDuplicatePackageFile = util.NewAlreadyExistErrorf("package file already exists")
	// ErrPackageFileNotExist indicates a package file not exist error
	ErrPackageFileNotExist = util.NewNotExistErrorf("package file does not exist")
)

// EmptyFileKey is a named constant for an empty file key
const EmptyFileKey = ""

// PackageFile represents a package file
type PackageFile struct {
	ID           int64              `xorm:"pk autoincr"`
	VersionID    int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	BlobID       int64              `xorm:"INDEX NOT NULL"`
	Name         string             `xorm:"NOT NULL"`
	LowerName    string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
	CompositeKey string             `xorm:"UNIQUE(s) INDEX"`
	IsLead       bool               `xorm:"NOT NULL DEFAULT false"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
}

// TryInsertFile inserts a file. If the file exists already ErrDuplicatePackageFile is returned
func TryInsertFile(ctx context.Context, pf *PackageFile) (*PackageFile, error) {
	switch {
	case setting.Database.Type.IsMySQL():
		if _, err := db.GetEngine(ctx).Exec("INSERT INTO package_file (version_id,blob_id,name,lower_name,composite_key,is_lead) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE 1=1",
			pf.VersionID, pf.BlobID, pf.Name, pf.LowerName, pf.CompositeKey, pf.IsLead); err != nil {
			return nil, err
		}
	case setting.Database.Type.IsPostgreSQL(), setting.Database.Type.IsSQLite3():
		if _, err := db.GetEngine(ctx).Exec("INSERT INTO package_file (version_id,blob_id,name,lower_name,composite_key,is_lead) VALUES (?,?,?,?,?,?) ON CONFLICT (version_id,lower_name,composite_key) DO UPDATE SET lower_name=lower_name",
			pf.VersionID, pf.BlobID, pf.Name, pf.LowerName, pf.CompositeKey, pf.IsLead); err != nil {
			return nil, err
		}
	case setting.Database.Type.IsMSSQL():
		r := func(s string) string {
			return strings.ReplaceAll(s, "'", "''")
		}
		sql := fmt.Sprintf(`
		MERGE INTO package_file WITH (HOLDLOCK) AS target USING (
			SELECT
				%d AS version_id,
				%d AS blob_id,
				'%s' AS name,
				'%s' AS lower_name,
				'%s' AS composite_key,
				%s AS is_lead
			) AS source (
				version_id, blob_id, name, lower_name, composite_key, is_lead
			) ON (
				target.version_id = source.version_id
  			AND target.lower_name = source.lower_name
				AND target.composite_key = source.composite_key
			) WHEN MATCHED
  				THEN UPDATE SET 1 = 1
				WHEN NOT MATCHED
  				THEN INSERT (
						version_id, blob_id, name, lower_name, composite_key, is_lead
					) VALUES (
						%d, %d, '%s', '%s', '%s', %s
					)`,
			pf.VersionID, pf.BlobID, r(pf.Name), r(pf.LowerName), r(pf.CompositeKey), strconv.FormatBool(pf.IsLead),
			pf.VersionID, pf.BlobID, r(pf.Name), r(pf.LowerName), r(pf.CompositeKey), strconv.FormatBool(pf.IsLead),
		)

		if _, err := db.GetEngine(ctx).Exec(sql); err != nil {
			return nil, err
		}
	}

	existing := &PackageFile{}
	has, err := db.GetEngine(ctx).Where(builder.Eq{
		"version_id":    pf.VersionID,
		"lower_name":    pf.LowerName,
		"composite_key": pf.CompositeKey,
	}).Get(existing)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.ErrNotExist
	}
	return existing, nil
}

// GetFilesByVersionID gets all files of a version
func GetFilesByVersionID(ctx context.Context, versionID int64) ([]*PackageFile, error) {
	pfs := make([]*PackageFile, 0, 10)
	return pfs, db.GetEngine(ctx).Where("version_id = ?", versionID).Find(&pfs)
}

// GetFileForVersionByID gets a file of a version by id
func GetFileForVersionByID(ctx context.Context, versionID, fileID int64) (*PackageFile, error) {
	pf := &PackageFile{
		VersionID: versionID,
	}

	has, err := db.GetEngine(ctx).ID(fileID).Get(pf)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageFileNotExist
	}
	return pf, nil
}

// GetFileForVersionByName gets a file of a version by name
func GetFileForVersionByName(ctx context.Context, versionID int64, name, key string) (*PackageFile, error) {
	if name == "" {
		return nil, ErrPackageFileNotExist
	}

	pf := &PackageFile{}

	has, err := db.GetEngine(ctx).Where(builder.Eq{
		"version_id":    versionID,
		"lower_name":    strings.ToLower(name),
		"composite_key": key,
	}).Get(pf)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPackageFileNotExist
	}
	return pf, nil
}

// DeleteFileByID deletes a file
func DeleteFileByID(ctx context.Context, fileID int64) error {
	_, err := db.GetEngine(ctx).ID(fileID).Delete(&PackageFile{})
	return err
}

// PackageFileSearchOptions are options for SearchXXX methods
type PackageFileSearchOptions struct {
	OwnerID       int64
	PackageType   Type
	VersionID     int64
	Query         string
	CompositeKey  string
	Properties    map[string]string
	OlderThan     time.Duration
	HashAlgorithm string
	Hash          string
	db.Paginator
}

func (opts *PackageFileSearchOptions) toConds() builder.Cond {
	cond := builder.NewCond()

	if opts.VersionID != 0 {
		cond = cond.And(builder.Eq{"package_file.version_id": opts.VersionID})
	} else if opts.OwnerID != 0 || (opts.PackageType != "" && opts.PackageType != "all") {
		var versionCond builder.Cond = builder.Eq{
			"package_version.is_internal": false,
		}
		if opts.OwnerID != 0 {
			versionCond = versionCond.And(builder.Eq{"package.owner_id": opts.OwnerID})
		}
		if opts.PackageType != "" && opts.PackageType != "all" {
			versionCond = versionCond.And(builder.Eq{"package.type": opts.PackageType})
		}

		in := builder.
			Select("package_version.id").
			From("package_version").
			InnerJoin("package", "package.id = package_version.package_id").
			Where(versionCond)

		cond = cond.And(builder.In("package_file.version_id", in))
	}
	if opts.CompositeKey != "" {
		cond = cond.And(builder.Eq{"package_file.composite_key": opts.CompositeKey})
	}
	if opts.Query != "" {
		cond = cond.And(builder.Like{"package_file.lower_name", strings.ToLower(opts.Query)})
	}

	if len(opts.Properties) != 0 {
		var propsCond builder.Cond = builder.Eq{
			"package_property.ref_type": PropertyTypeFile,
		}
		propsCond = propsCond.And(builder.Expr("package_property.ref_id = package_file.id"))

		propsCondBlock := builder.NewCond()
		for name, value := range opts.Properties {
			propsCondBlock = propsCondBlock.Or(builder.Eq{
				"package_property.name":  name,
				"package_property.value": value,
			})
		}
		propsCond = propsCond.And(propsCondBlock)

		cond = cond.And(builder.Eq{
			strconv.Itoa(len(opts.Properties)): builder.Select("COUNT(*)").Where(propsCond).From("package_property"),
		})
	}

	if opts.OlderThan != 0 {
		cond = cond.And(builder.Lt{"package_file.created_unix": time.Now().Add(-opts.OlderThan).Unix()})
	}

	if opts.Hash != "" {
		var field string
		switch strings.ToLower(opts.HashAlgorithm) {
		case "md5":
			field = "package_blob.hash_md5"
		case "sha1":
			field = "package_blob.hash_sha1"
		case "sha256":
			field = "package_blob.hash_sha256"
		case "sha512":
			fallthrough
		default: // default to SHA512 if not specified or unknown
			field = "package_blob.hash_sha512"
		}
		innerCond := builder.
			Expr("package_blob.id = package_file.blob_id").
			And(builder.Eq{field: opts.Hash})
		cond = cond.And(builder.Exists(builder.Select("package_blob.id").From("package_blob").Where(innerCond)))
	}

	return cond
}

// SearchFiles gets all files of packages matching the search options
func SearchFiles(ctx context.Context, opts *PackageFileSearchOptions) ([]*PackageFile, int64, error) {
	sess := db.GetEngine(ctx).
		Where(opts.toConds())

	if opts.Paginator != nil {
		sess = db.SetSessionPagination(sess, opts)
	}

	pfs := make([]*PackageFile, 0, 10)
	count, err := sess.FindAndCount(&pfs)
	return pfs, count, err
}

// CalculateFileSize sums up all blob sizes matching the search options.
// It does NOT respect the deduplication of blobs.
func CalculateFileSize(ctx context.Context, opts *PackageFileSearchOptions) (int64, error) {
	return db.GetEngine(ctx).
		Table("package_file").
		Where(opts.toConds()).
		Join("INNER", "package_blob", "package_blob.id = package_file.blob_id").
		SumInt(new(PackageBlob), "size")
}

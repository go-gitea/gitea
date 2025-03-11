// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(PackageFile))
}

var (
	// ErrMetadataFile indicated a metadata file
	ErrMetadataFile = errors.New("metadata file")
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
	e := db.GetEngine(ctx)

	existing := &PackageFile{}

	has, err := e.Where(builder.Eq{
		"version_id":    pf.VersionID,
		"lower_name":    pf.LowerName,
		"composite_key": pf.CompositeKey,
	}).Get(existing)
	if err != nil {
		return nil, err
	}
	if has {
		return existing, ErrDuplicatePackageFile
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

// HasFiles tests if there are files of packages matching the search options
func HasFiles(ctx context.Context, opts *PackageFileSearchOptions) (bool, error) {
	return db.Exist[PackageFile](ctx, opts.toConds())
}

// GetFilesBelowBuildNumber retrieves all files for maven snapshot version where the build number is <= maxBuildNumber.
// Returns two slices: one for filtered files and one for skipped files.
func GetFilesBelowBuildNumber(ctx context.Context, versionID int64, maxBuildNumber int, classifiers ...string) ([]*PackageFile, []*PackageFile, error) {
	if maxBuildNumber <= 0 {
		return nil, nil, errors.New("maxBuildNumber must be a positive integer")
	}

	files, err := GetFilesByVersionID(ctx, versionID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve files: %w", err)
	}

	var filteredFiles, skippedFiles []*PackageFile
	for _, file := range files {
		buildNumber, err := extractBuildNumberFromFileName(file.Name, classifiers...)
		if err != nil {
			if !errors.Is(err, ErrMetadataFile) {
				skippedFiles = append(skippedFiles, file)
			}
			continue
		}
		if buildNumber <= maxBuildNumber {
			filteredFiles = append(filteredFiles, file)
		}
	}

	return filteredFiles, skippedFiles, nil
}

// extractBuildNumberFromFileName extracts the build number from a Maven snapshot file name.
// Expected formats:
//
//	"artifact-1.0.0-20250311.083409-9.tgz" returns 9
//	"artifact-to-test-2.0.0-20250311.083409-10-sources.tgz" returns 10
func extractBuildNumberFromFileName(filename string, classifiers ...string) (int, error) {
	if strings.Contains(filename, "maven-metadata.xml") {
		return 0, ErrMetadataFile
	}

	dotIdx := strings.LastIndex(filename, ".")
	if dotIdx == -1 {
		return 0, fmt.Errorf("extract build number from filename: no file extension found in '%s'", filename)
	}
	base := filename[:dotIdx]

	// Remove classifier suffix if present.
	for _, classifier := range classifiers {
		suffix := "-" + classifier
		if strings.HasSuffix(base, suffix) {
			base = base[:len(base)-len(suffix)]
			break
		}
	}

	// The build number should be the token after the last dash.
	lastDash := strings.LastIndex(base, "-")
	if lastDash == -1 {
		return 0, fmt.Errorf("extract build number from filename: invalid file name format in '%s'", filename)
	}
	buildNumberStr := base[lastDash+1:]
	buildNumber, err := strconv.Atoi(buildNumberStr)
	if err != nil {
		return 0, fmt.Errorf("extract build number from filename: failed to convert build number '%s' to integer in '%s': %v", buildNumberStr, filename, err)
	}

	return buildNumber, nil
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

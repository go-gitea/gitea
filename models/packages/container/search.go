// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package container

import (
	"context"
	"errors"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	container_module "code.gitea.io/gitea/modules/packages/container"

	"xorm.io/builder"
)

var ErrContainerBlobNotExist = errors.New("Container blob does not exist")

type BlobSearchOptions struct {
	OwnerID    int64
	Image      string
	Digest     string
	Tag        string
	IsManifest bool
}

func (opts *BlobSearchOptions) toConds() builder.Cond {
	var cond builder.Cond = builder.Eq{
		"package.type": packages.TypeContainer,
	}

	if opts.OwnerID != 0 {
		cond = cond.And(builder.Eq{"package.owner_id": opts.OwnerID})
	}
	if opts.Image != "" {
		cond = cond.And(builder.Eq{"package.lower_name": strings.ToLower(opts.Image)})
	}
	if opts.Tag != "" {
		cond = cond.And(builder.Eq{"package_version.lower_version": strings.ToLower(opts.Tag)})
	}
	if opts.IsManifest {
		cond = cond.And(builder.Eq{"package_file.lower_name": ManifestFilename})
	}
	if opts.Digest != "" {
		var propsCond builder.Cond = builder.Eq{
			"package_property.ref_type": packages.PropertyTypeFile,
			"package_property.name":     container_module.PropertyDigest,
			"package_property.value":    opts.Digest,
		}

		cond = cond.And(builder.In("package_file.id", builder.Select("package_property.ref_id").Where(propsCond).From("package_property")))
	}

	return cond
}

// GetContainerBlob gets the container blob matching the blob search options
// If multiple matching blobs are found (manifests with the same digest) the first (according to the database) is selected.
func GetContainerBlob(ctx context.Context, opts *BlobSearchOptions) (*packages.PackageFileDescriptor, error) {
	pfds, err := getContainerBlobsLimit(ctx, opts, 1)
	if err != nil {
		return nil, err
	}
	if len(pfds) != 1 {
		return nil, ErrContainerBlobNotExist
	}

	return pfds[0], nil
}

// GetContainerBlobs gets the container blobs matching the blob search options
func GetContainerBlobs(ctx context.Context, opts *BlobSearchOptions) ([]*packages.PackageFileDescriptor, error) {
	return getContainerBlobsLimit(ctx, opts, 0)
}

func getContainerBlobsLimit(ctx context.Context, opts *BlobSearchOptions, limit int) ([]*packages.PackageFileDescriptor, error) {
	pfs := make([]*packages.PackageFile, 0, limit)
	sess := db.GetEngine(ctx).
		Join("INNER", "package_version", "package_version.id = package_file.version_id").
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(opts.toConds())

	if limit > 0 {
		sess = sess.Limit(limit)
	}

	if err := sess.Find(&pfs); err != nil {
		return nil, err
	}

	pfds := make([]*packages.PackageFileDescriptor, 0, len(pfs))
	for _, pf := range pfs {
		pfd, err := packages.GetPackageFileDescriptor(ctx, pf)
		if err != nil {
			return nil, err
		}
		pfds = append(pfds, pfd)
	}

	return pfds, nil
}

// GetManifestVersions gets all package versions representing the matching manifest
func GetManifestVersions(ctx context.Context, opts *BlobSearchOptions) ([]*packages.PackageVersion, error) {
	cond := opts.toConds().And(builder.Eq{"package_version.is_internal": false})

	pvs := make([]*packages.PackageVersion, 0, 10)
	return pvs, db.GetEngine(ctx).
		Join("INNER", "package", "package.id = package_version.package_id").
		Join("INNER", "package_file", "package_file.version_id = package_version.id").
		Where(cond).
		Find(&pvs)
}

// GetImageTags gets a sorted list of the tags of an image
// The result is suitable for the api call.
func GetImageTags(ctx context.Context, ownerID int64, image string, n int, last string) ([]string, error) {
	// Short circuit: n == 0 should return an empty list
	if n == 0 {
		return []string{}, nil
	}

	var cond builder.Cond = builder.Eq{
		"package.type":                packages.TypeContainer,
		"package.owner_id":            ownerID,
		"package.lower_name":          strings.ToLower(image),
		"package_version.is_internal": false,
	}

	var propsCond builder.Cond = builder.Eq{
		"package_property.ref_type": packages.PropertyTypeVersion,
		"package_property.name":     container_module.PropertyManifestTagged,
	}

	cond = cond.And(builder.In("package_version.id", builder.Select("package_property.ref_id").Where(propsCond).From("package_property")))

	if last != "" {
		cond = cond.And(builder.Gt{"package_version.lower_version": strings.ToLower(last)})
	}

	sess := db.GetEngine(ctx).
		Table("package_version").
		Select("package_version.lower_version").
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(cond).
		Asc("package_version.lower_version")

	var tags []string
	if n > 0 {
		sess = sess.Limit(n)

		tags = make([]string, 0, n)
	} else {
		tags = make([]string, 0, 10)
	}

	return tags, sess.Find(&tags)
}

type ImageTagsSearchOptions struct {
	PackageID int64
	Query     string
	IsTagged  bool
	db.Paginator
}

func (opts *ImageTagsSearchOptions) toConds() builder.Cond {
	var cond builder.Cond = builder.Eq{
		"package.type":                packages.TypeContainer,
		"package.id":                  opts.PackageID,
		"package_version.is_internal": false,
	}

	if opts.Query != "" {
		cond = cond.And(builder.Like{"package_version.lower_version", strings.ToLower(opts.Query)})
	}

	var propsCond builder.Cond = builder.Eq{
		"package_property.ref_type": packages.PropertyTypeVersion,
		"package_property.name":     container_module.PropertyManifestTagged,
	}

	in := builder.In("package_version.id", builder.Select("package_property.ref_id").Where(propsCond).From("package_property"))

	if opts.IsTagged {
		cond = cond.And(in)
	} else {
		cond = cond.And(builder.Not{in})
	}

	return cond
}

// SearchImageTags gets a sorted list of the tags of an image
func SearchImageTags(ctx context.Context, opts *ImageTagsSearchOptions) ([]*packages.PackageVersion, int64, error) {
	sess := db.GetEngine(ctx).
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(opts.toConds()).
		Desc("package_version.created_unix")

	if opts.Paginator != nil {
		sess = db.SetSessionPagination(sess, opts)
	}

	pvs := make([]*packages.PackageVersion, 0, 10)
	count, err := sess.FindAndCount(&pvs)
	return pvs, count, err
}

func SearchExpiredUploadedBlobs(ctx context.Context, olderThan time.Duration) ([]*packages.PackageFile, error) {
	var cond builder.Cond = builder.Eq{
		"package_version.is_internal":   true,
		"package_version.lower_version": UploadVersion,
		"package.type":                  packages.TypeContainer,
	}
	cond = cond.And(builder.Lt{"package_file.created_unix": time.Now().Add(-olderThan).Unix()})

	var pfs []*packages.PackageFile
	return pfs, db.GetEngine(ctx).
		Join("INNER", "package_version", "package_version.id = package_file.version_id").
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(cond).
		Find(&pfs)
}

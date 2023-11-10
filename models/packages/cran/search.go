// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cran

import (
	"context"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	cran_module "code.gitea.io/gitea/modules/packages/cran"

	"xorm.io/builder"
)

type SearchOptions struct {
	OwnerID  int64
	FileType string
	Platform string
	RVersion string
	Filename string
}

func (opts *SearchOptions) toConds() builder.Cond {
	var cond builder.Cond = builder.Eq{
		"package.type":                packages.TypeCran,
		"package.owner_id":            opts.OwnerID,
		"package_version.is_internal": false,
	}

	if opts.Filename != "" {
		cond = cond.And(builder.Eq{"package_file.lower_name": strings.ToLower(opts.Filename)})
	}

	var propsCond builder.Cond = builder.Eq{
		"package_property.ref_type": packages.PropertyTypeFile,
	}
	propsCond = propsCond.And(builder.Expr("package_property.ref_id = package_file.id"))

	count := 1
	propsCondBlock := builder.Eq{"package_property.name": cran_module.PropertyType}.And(builder.Eq{"package_property.value": opts.FileType})

	if opts.Platform != "" {
		count += 2
		propsCondBlock = propsCondBlock.
			Or(builder.Eq{"package_property.name": cran_module.PropertyPlatform}.And(builder.Eq{"package_property.value": opts.Platform})).
			Or(builder.Eq{"package_property.name": cran_module.PropertyRVersion}.And(builder.Eq{"package_property.value": opts.RVersion}))
	}

	propsCond = propsCond.And(propsCondBlock)

	cond = cond.And(builder.Eq{
		strconv.Itoa(count): builder.Select("COUNT(*)").Where(propsCond).From("package_property"),
	})

	return cond
}

func SearchLatestVersions(ctx context.Context, opts *SearchOptions) ([]*packages.PackageVersion, error) {
	sess := db.GetEngine(ctx).
		Table("package_version").
		Select("package_version.*").
		Join("LEFT", "package_version pv2", builder.Expr("package_version.package_id = pv2.package_id AND pv2.is_internal = ? AND (package_version.created_unix < pv2.created_unix OR (package_version.created_unix = pv2.created_unix AND package_version.id < pv2.id))", false)).
		Join("INNER", "package", "package.id = package_version.package_id").
		Join("INNER", "package_file", "package_file.version_id = package_version.id").
		Where(opts.toConds().And(builder.Expr("pv2.id IS NULL"))).
		Asc("package.name")

	pvs := make([]*packages.PackageVersion, 0, 10)
	return pvs, sess.Find(&pvs)
}

func SearchFile(ctx context.Context, opts *SearchOptions) (*packages.PackageFile, error) {
	sess := db.GetEngine(ctx).
		Table("package_version").
		Select("package_file.*").
		Join("INNER", "package", "package.id = package_version.package_id").
		Join("INNER", "package_file", "package_file.version_id = package_version.id").
		Where(opts.toConds())

	pf := &packages.PackageFile{}
	if has, err := sess.Get(pf); err != nil {
		return nil, err
	} else if !has {
		return nil, packages.ErrPackageFileNotExist
	}
	return pf, nil
}

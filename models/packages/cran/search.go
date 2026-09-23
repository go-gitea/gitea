// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cran

import (
	"context"
	"strconv"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/packages"
	cran_module "gitea.dev/modules/packages/cran"

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
		Join("INNER", "package", "package.id = package_version.package_id").
		Join("INNER", "package_file", "package_file.version_id = package_version.id").
		Where(opts.toConds()).
		Asc("package.name").
		Desc("package_version.created_unix", "package_version.id")

	pvs := make([]*packages.PackageVersion, 0, 10)
	rows, err := sess.Rows(new(packages.PackageVersion))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indices := make(map[int64]int)
	for rows.Next() {
		pv := new(packages.PackageVersion)
		if err := rows.Scan(pv); err != nil {
			return nil, err
		}
		if i, ok := indices[pv.PackageID]; !ok {
			indices[pv.PackageID] = len(pvs)
			pvs = append(pvs, pv)
		} else if cran_module.CompareVersions(pv.Version, pvs[i].Version) > 0 {
			pvs[i] = pv
		}
	}
	return pvs, rows.Err()
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

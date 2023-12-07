// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"

	"xorm.io/builder"
)

// SearchVersions gets all versions of packages matching the search options
func SearchVersions(ctx context.Context, opts *packages_model.PackageSearchOptions) ([]*packages_model.PackageVersion, int64, error) {
	cond := toConds(opts)

	e := db.GetEngine(ctx)

	total, err := e.
		Where(cond).
		Count(&packages_model.Package{})
	if err != nil {
		return nil, 0, err
	}

	inner := builder.
		Dialect(db.BuilderDialect()). // builder needs the sql dialect to build the Limit() below
		Select("*").
		From("package").
		Where(cond).
		OrderBy("package.name ASC")
	if opts.Paginator != nil {
		skip, take := opts.GetSkipTake()
		inner = inner.Limit(take, skip)
	}

	sess := e.
		Where(opts.ToConds()).
		Table("package_version").
		Join("INNER", inner, "package.id = package_version.package_id")

	pvs := make([]*packages_model.PackageVersion, 0, 10)
	return pvs, total, sess.Find(&pvs)
}

// CountPackages counts all packages matching the search options
func CountPackages(ctx context.Context, opts *packages_model.PackageSearchOptions) (int64, error) {
	return db.GetEngine(ctx).
		Where(toConds(opts)).
		Count(&packages_model.Package{})
}

func toConds(opts *packages_model.PackageSearchOptions) builder.Cond {
	var cond builder.Cond = builder.Eq{
		"package.is_internal": opts.IsInternal.IsTrue(),
		"package.owner_id":    opts.OwnerID,
		"package.type":        packages_model.TypeNuGet,
	}
	if opts.Name.Value != "" {
		if opts.Name.ExactMatch {
			cond = cond.And(builder.Eq{"package.lower_name": strings.ToLower(opts.Name.Value)})
		} else {
			cond = cond.And(builder.Like{"package.lower_name", strings.ToLower(opts.Name.Value)})
		}
	}
	return cond
}

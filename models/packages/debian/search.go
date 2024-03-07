// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"context"
	"strconv"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	debian_module "code.gitea.io/gitea/modules/packages/debian"

	"xorm.io/builder"
)

type PackageSearchOptions struct {
	OwnerID      int64
	Distribution string
	Component    string
	Architecture string
}

func (opts *PackageSearchOptions) toCond() builder.Cond {
	var cond builder.Cond = builder.Eq{
		"package_file.is_lead":        true,
		"package.type":                packages.TypeDebian,
		"package.owner_id":            opts.OwnerID,
		"package.is_internal":         false,
		"package_version.is_internal": false,
	}

	props := make(map[string]string)
	if opts.Distribution != "" {
		props[debian_module.PropertyDistribution] = opts.Distribution
	}
	if opts.Component != "" {
		props[debian_module.PropertyComponent] = opts.Component
	}
	if opts.Architecture != "" {
		props[debian_module.PropertyArchitecture] = opts.Architecture
	}

	if len(props) > 0 {
		var propsCond builder.Cond = builder.Eq{
			"package_property.ref_type": packages.PropertyTypeFile,
		}
		propsCond = propsCond.And(builder.Expr("package_property.ref_id = package_file.id"))

		propsCondBlock := builder.NewCond()
		for name, value := range props {
			propsCondBlock = propsCondBlock.Or(builder.Eq{
				"package_property.name":  name,
				"package_property.value": value,
			})
		}
		propsCond = propsCond.And(propsCondBlock)

		cond = cond.And(builder.Eq{
			strconv.Itoa(len(props)): builder.Select("COUNT(*)").Where(propsCond).From("package_property"),
		})
	}

	return cond
}

// ExistPackages tests if there are packages matching the search options
func ExistPackages(ctx context.Context, opts *PackageSearchOptions) (bool, error) {
	return db.GetEngine(ctx).
		Table("package_file").
		Join("INNER", "package_version", "package_version.id = package_file.version_id").
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(opts.toCond()).
		Exist(new(packages.PackageFile))
}

// SearchPackages gets the packages matching the search options
func SearchPackages(ctx context.Context, opts *PackageSearchOptions, iter func(*packages.PackageFileDescriptor)) error {
	return db.GetEngine(ctx).
		Table("package_file").
		Select("package_file.*").
		Join("INNER", "package_version", "package_version.id = package_file.version_id").
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(opts.toCond()).
		Asc("package.lower_name", "package_version.created_unix").
		Iterate(new(packages.PackageFile), func(_ int, bean any) error {
			pf := bean.(*packages.PackageFile)

			pfd, err := packages.GetPackageFileDescriptor(ctx, pf)
			if err != nil {
				return err
			}

			iter(pfd)

			return nil
		})
}

// GetDistributions gets all available distributions
func GetDistributions(ctx context.Context, ownerID int64) ([]string, error) {
	return packages.GetDistinctPropertyValues(
		ctx,
		packages.TypeDebian,
		ownerID,
		packages.PropertyTypeFile,
		debian_module.PropertyDistribution,
		nil,
	)
}

// GetComponents gets all available components for the given distribution
func GetComponents(ctx context.Context, ownerID int64, distribution string) ([]string, error) {
	return packages.GetDistinctPropertyValues(
		ctx,
		packages.TypeDebian,
		ownerID,
		packages.PropertyTypeFile,
		debian_module.PropertyComponent,
		&packages.DistinctPropertyDependency{
			Name:  debian_module.PropertyDistribution,
			Value: distribution,
		},
	)
}

// GetArchitectures gets all available architectures for the given distribution
func GetArchitectures(ctx context.Context, ownerID int64, distribution string) ([]string, error) {
	return packages.GetDistinctPropertyValues(
		ctx,
		packages.TypeDebian,
		ownerID,
		packages.PropertyTypeFile,
		debian_module.PropertyArchitecture,
		&packages.DistinctPropertyDependency{
			Name:  debian_module.PropertyDistribution,
			Value: distribution,
		},
	)
}

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

// SearchLatestPackages gets the latest packages matching the search options
func SearchLatestPackages(ctx context.Context, opts *PackageSearchOptions) ([]*packages.PackageFileDescriptor, error) {
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

	cond = cond.
		And(builder.Expr("pv2.id IS NULL"))

	joinCond := builder.
		Expr("package_version.package_id = pv2.package_id AND (package_version.created_unix < pv2.created_unix OR (package_version.created_unix = pv2.created_unix AND package_version.id < pv2.id))").
		And(builder.Eq{"pv2.is_internal": false})

	pfs := make([]*packages.PackageFile, 0, 10)
	err := db.GetEngine(ctx).
		Table("package_file").
		Select("package_file.*").
		Join("INNER", "package_version", "package_version.id = package_file.version_id").
		Join("LEFT", "package_version pv2", joinCond).
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(cond).
		Desc("package_version.created_unix").
		Find(&pfs)
	if err != nil {
		return nil, err
	}

	return packages.GetPackageFileDescriptors(ctx, pfs)
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

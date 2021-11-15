// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"

	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(PackageVersionProperty))
}

// PackageVersionProperty represents a property of a package version
type PackageVersionProperty struct {
	ID        int64  `xorm:"pk autoincr"`
	VersionID int64  `xorm:"INDEX NOT NULL"`
	Name      string `xorm:"INDEX NOT NULL"`
	Value     string `xorm:"INDEX NOT NULL"`
}

// InsertVersionProperty creates a property
func InsertVersionProperty(ctx context.Context, versionID int64, name, value string) (*PackageVersionProperty, error) {
	pvp := &PackageVersionProperty{
		VersionID: versionID,
		Name:      name,
		Value:     value,
	}

	_, err := db.GetEngine(ctx).Insert(pvp)
	return pvp, err
}

// GetVersionProperties gets all properties of the version
func GetVersionProperties(ctx context.Context, versionID int64) ([]*PackageVersionProperty, error) {
	pvps := make([]*PackageVersionProperty, 0, 10)
	return pvps, db.GetEngine(ctx).Where("version_id = ?", versionID).Find(&pvps)
}

// GetVersionPropertiesByName gets all properties with a specific name of the version
func GetVersionPropertiesByName(ctx context.Context, versionID int64, name string) ([]*PackageVersionProperty, error) {
	pvps := make([]*PackageVersionProperty, 0, 10)
	return pvps, db.GetEngine(ctx).Where("version_id = ? AND name = ?", versionID, name).Find(&pvps)
}

// FindVersionsByPropertyNameAndValue gets all package versions which are associated with a specific property + value
func FindVersionsByPropertyNameAndValue(ctx context.Context, packageID int64, name, value string) ([]*PackageVersion, error) {
	var cond builder.Cond = builder.Eq{
		"package_version_property.name":  name,
		"package_version_property.value": value,
		"package_version.package_id":     packageID,
	}

	pvs := make([]*PackageVersion, 0, 5)
	return pvs, db.GetEngine(ctx).
		Where(cond).
		Join("INNER", "package_version_property", "package_version_property.version_id = package_version.id").
		Find(&pvs)
}

// DeleteVersionPropertiesByVersionID deletes all properties of the version
func DeleteVersionPropertiesByVersionID(ctx context.Context, versionID int64) error {
	_, err := db.GetEngine(ctx).Where("version_id = ?", versionID).Delete(&PackageVersionProperty{})
	return err
}

// DeleteVersionPropertyByID deletes a property
func DeleteVersionPropertyByID(ctx context.Context, propertyID int64) error {
	_, err := db.GetEngine(ctx).ID(propertyID).Delete(&PackageVersionProperty{})
	return err
}

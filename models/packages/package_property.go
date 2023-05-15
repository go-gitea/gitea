// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"

	"code.gitea.io/gitea/models/db"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(PackageProperty))
}

type PropertyType int64

const (
	// PropertyTypeVersion means the reference is a package version
	PropertyTypeVersion PropertyType = iota // 0
	// PropertyTypeFile means the reference is a package file
	PropertyTypeFile // 1
	// PropertyTypePackage means the reference is a package
	PropertyTypePackage // 2
)

// PackageProperty represents a property of a package, version or file
type PackageProperty struct {
	ID      int64        `xorm:"pk autoincr"`
	RefType PropertyType `xorm:"INDEX NOT NULL"`
	RefID   int64        `xorm:"INDEX NOT NULL"`
	Name    string       `xorm:"INDEX NOT NULL"`
	Value   string       `xorm:"TEXT NOT NULL"`
}

// InsertProperty creates a property
func InsertProperty(ctx context.Context, refType PropertyType, refID int64, name, value string) (*PackageProperty, error) {
	pp := &PackageProperty{
		RefType: refType,
		RefID:   refID,
		Name:    name,
		Value:   value,
	}

	_, err := db.GetEngine(ctx).Insert(pp)
	return pp, err
}

// GetProperties gets all properties
func GetProperties(ctx context.Context, refType PropertyType, refID int64) ([]*PackageProperty, error) {
	pps := make([]*PackageProperty, 0, 10)
	return pps, db.GetEngine(ctx).Where("ref_type = ? AND ref_id = ?", refType, refID).Find(&pps)
}

// GetPropertiesByName gets all properties with a specific name
func GetPropertiesByName(ctx context.Context, refType PropertyType, refID int64, name string) ([]*PackageProperty, error) {
	pps := make([]*PackageProperty, 0, 10)
	return pps, db.GetEngine(ctx).Where("ref_type = ? AND ref_id = ? AND name = ?", refType, refID, name).Find(&pps)
}

// UpdateProperty updates a property
func UpdateProperty(ctx context.Context, pp *PackageProperty) error {
	_, err := db.GetEngine(ctx).ID(pp.ID).Update(pp)
	return err
}

// DeleteAllProperties deletes all properties of a ref
func DeleteAllProperties(ctx context.Context, refType PropertyType, refID int64) error {
	_, err := db.GetEngine(ctx).Where("ref_type = ? AND ref_id = ?", refType, refID).Delete(&PackageProperty{})
	return err
}

// DeletePropertyByID deletes a property
func DeletePropertyByID(ctx context.Context, propertyID int64) error {
	_, err := db.GetEngine(ctx).ID(propertyID).Delete(&PackageProperty{})
	return err
}

// DeletePropertyByName deletes properties by name
func DeletePropertyByName(ctx context.Context, refType PropertyType, refID int64, name string) error {
	_, err := db.GetEngine(ctx).Where("ref_type = ? AND ref_id = ? AND name = ?", refType, refID, name).Delete(&PackageProperty{})
	return err
}

type DistinctPropertyDependency struct {
	Name  string
	Value string
}

// GetDistinctPropertyValues returns all distinct property values for a given type.
// Optional: Search only in dependence of another property.
func GetDistinctPropertyValues(ctx context.Context, packageType Type, ownerID int64, refType PropertyType, propertyName string, dep *DistinctPropertyDependency) ([]string, error) {
	var cond builder.Cond = builder.Eq{
		"package_property.ref_type": refType,
		"package_property.name":     propertyName,
		"package.type":              packageType,
		"package.owner_id":          ownerID,
	}
	if dep != nil {
		innerCond := builder.
			Expr("pp.ref_id = package_property.ref_id").
			And(builder.Eq{
				"pp.ref_type": refType,
				"pp.name":     dep.Name,
				"pp.value":    dep.Value,
			})
		cond = cond.And(builder.Exists(builder.Select("pp.ref_id").From("package_property pp").Where(innerCond)))
	}

	values := make([]string, 0, 5)
	return values, db.GetEngine(ctx).
		Table("package_property").
		Distinct("package_property.value").
		Join("INNER", "package_file", "package_file.id = package_property.ref_id").
		Join("INNER", "package_version", "package_version.id = package_file.version_id").
		Join("INNER", "package", "package.id = package_version.package_id").
		Where(cond).
		Find(&values)
}

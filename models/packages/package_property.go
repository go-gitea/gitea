// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"

	"code.gitea.io/gitea/models/db"
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

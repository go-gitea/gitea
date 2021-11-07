// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package packages

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

func init() {
	db.RegisterModel(new(PackageVersionProperty))
}

// PackageVersionProperty represents a property of a package version
type PackageVersionProperty struct {
	ID        int64  `xorm:"pk autoincr"`
	VersionID int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name      string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Value     string `xorm:"INDEX NOT NULL"`
}

// SetVersionProperties sets the properties of the version
func SetVersionProperties(ctx context.Context, versionID int64, properties map[string]string) error {
	e := db.GetEngine(ctx)

	for name, value := range properties {
		pvp := &PackageVersionProperty{
			VersionID: versionID,
			Name:      name,
			Value:     value,
		}

		count, err := e.Where("version_id = ? AND name = ?", versionID, pvp.Name).Update(pvp)
		if err != nil {
			return err
		}
		if count != 1 {
			if _, err = e.Insert(pvp); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetVersionProperties gets all properties of the version
func GetVersionProperties(ctx context.Context, versionID int64) ([]*PackageVersionProperty, error) {
	pvps := make([]*PackageVersionProperty, 0, 10)
	return pvps, db.GetEngine(ctx).Where("version_id = ?", versionID).Find(&pvps)
}

// DeleteVersionPropertiesByVersionID deletes all properties of the version
func DeleteVersionPropertiesByVersionID(ctx context.Context, versionID int64) error {
	_, err := db.GetEngine(ctx).Where("version_id = ?", versionID).Delete(&PackageVersionProperty{})
	return err
}

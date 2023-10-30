// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/rpm"

	"xorm.io/xorm"
)

func RebuildRpmPackage(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	// NOTE: package_property[name='rpm.metadata'] is very large,
	// and to avoid querying all of them resulting in large memory,
	// a single RPM package is now used for updating.
	metadata := make([][]string, 0, 4)
	_, err := sess.Cols("ref_type", "ref_id", "package_file.composite_key", "value").
		Table("package_property").
		Join("left", "package_file", "`package_file`.id = `package_property`.ref_id").
		Where("package_property.name = 'rpm.metadata'").Get(&metadata)
	if err != nil {
		return err
	}
	for _, data := range metadata {
		var rpmMetadata rpm.FileMetadata
		err = json.Unmarshal([]byte(data[3]), &rpmMetadata)
		if err != nil {
			return err
		}
		_, err = sess.Exec(
			"INSERT INTO package_property(ref_type, ref_id, name, value) values (?,?,?,?),(?,?,?,?)",
			data[0], data[1], rpm.PropertyGroup, data[2],
			data[0], data[1], rpm.PropertyArchitecture, rpmMetadata.Architecture,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

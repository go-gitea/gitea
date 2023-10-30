// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/rpm"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func RebuildRpmPackage(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	// group composite_key
	_, err := sess.Exec(`
	UPDATE package_file
	set composite_key = '/'
	WHERE package_file.lower_name like '%.rpm'
   	OR package_file.lower_name IN
    ('primary.xml.gz', 'other.xml.gz', 'filelists.xml.gz', 'other.xml.gz', 'repomd.xml', 'repomd.xml.asc')
	`)
	if err != nil {
		return err
	}
	// group version
	switch x.Dialect().URI().DBType {
	case schemas.SQLITE:
		_, err = sess.Exec(`update package_version
		set version       = '/' || version,
			lower_version = '/' || lower_version
		WHERE id IN (SELECT package_file.version_id as id
					 from package_file
					 WHERE package_file.name like '%.rpm')`)
	case schemas.MSSQL:
		_, err = sess.Exec(`update package_version
		set version       = '/' + version,
			lower_version = '/' + lower_version
		WHERE id IN (SELECT package_file.version_id as id
					 from package_file
					 WHERE package_file.name like '%.rpm')`)

	default:
		_, err = sess.Exec(`update package_version
						set version       = CONCAT('/',version),
							lower_version = CONCAT('/',lower_version)
						WHERE id IN (SELECT package_file.version_id as id
									 from package_file
									 WHERE package_file.name like '%.rpm')`)
	}
	if err != nil {
		return err
	}
	// NOTE: package_property[name='rpm.metadata'] is very large,
	// and to avoid querying all of them resulting in large memory,
	// a single RPM package is now used for updating.
	metadata := make([][]string, 0, 4)
	_, err = sess.Cols("ref_type", "ref_id", "package_file.composite_key", "value").
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

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/packages/rpm"

	"xorm.io/xorm"
)

func RebuildRpmPackage(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	defaultDistribution := rpm.RepositoryDefaultDistribution
	// select all old rpm package
	var oldRpmIds []int64
	ss := sess.Cols("id").
		Table("package_file").
		Where("composite_key = ?", "").
		And("lower_name like ?", "%.rpm")
	err := ss.Find(&oldRpmIds)
	if err != nil {
		return err
	}
	// add metadata
	// NOTE: package_property[name='rpm.metdata'] is very large,
	// and to avoid querying all of them resulting in large memory,
	// a single RPM package is now used for updating.
	for _, id := range oldRpmIds {

		metadata := make([]string, 0, 3)
		_, err := sess.Cols("ref_type", "ref_id", "value").
			Table("package_property").
			Where("name = 'rpm.metdata'").
			And("ref_id = ?", id).
			Get(&metadata)
		if err != nil {
			return err
		}
		// get rpm info
		var rpmMetadata rpm.FileMetadata
		err = json.Unmarshal([]byte(metadata[2]), &rpmMetadata)
		if err != nil {
			return err
		}
		_, err = sess.Exec(
			"INSERT INTO package_property(ref_type, ref_id, name, value) values (?,?,?,?),(?,?,?,?)",
			metadata[0], metadata[1], "rpm.distribution", defaultDistribution,
			metadata[0], metadata[1], "rpm.architecture", rpmMetadata.Architecture,
		)
		if err != nil {
			return err
		}
		// set default distribution
		_, err = sess.Table("package_file").
			Where("id = ?", id).
			Update(map[string]any{
				"composite_key": defaultDistribution,
			})
		if err != nil {
			return err
		}
	}
	// set old rpm index file to default distribution
	_, err = sess.Table("package_file").
		Where(
			"composite_key = '' AND " +
				"lower_name IN" +
				"(" +
				"'primary.xml.gz','other.xml.gz','filelists.xml.gz','other.xml.gz','repomd.xml','repomd.xml.asc'" +
				")").
		Update(map[string]any{
			"composite_key": defaultDistribution,
		})
	if err != nil {
		return err
	}
	return nil
}
func FixPackagePropertyTypo(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec(`UPDATE package_property SET name = 'rpm.metadata' WHERE name = 'rpm.metdata'`); err != nil {
		return err
	}
	if _, err := sess.Exec(`UPDATE package_property SET name = 'conda.metadata' WHERE name = 'conda.metdata'`); err != nil {
		return err
	}

	return sess.Commit()
}

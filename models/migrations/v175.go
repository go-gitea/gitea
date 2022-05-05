// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"regexp"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func fixPostgresIDSequences(x *xorm.Engine) error {
	if !setting.Database.UsePostgreSQL {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	var sequences []string
	schema := sess.Engine().Dialect().URI().Schema

	sess.Engine().SetSchema("")
	if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE 'tmp_recreate__%_id_seq%' AND sequence_catalog = ?", setting.Database.Name).Find(&sequences); err != nil {
		log.Error("Unable to find sequences: %v", err)
		return err
	}
	sess.Engine().SetSchema(schema)

	sequenceRegexp := regexp.MustCompile(`tmp_recreate__(\w+)_id_seq.*`)

	for _, sequence := range sequences {
		tableName := sequenceRegexp.FindStringSubmatch(sequence)[1]
		newSequenceName := tableName + "_id_seq"
		if _, err := sess.Exec(fmt.Sprintf("ALTER SEQUENCE `%s` RENAME TO `%s`", sequence, newSequenceName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", sequence, newSequenceName, err)
			return err
		}
		if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', COALESCE((SELECT MAX(id)+1 FROM `%s`), 1), false)", newSequenceName, tableName)); err != nil {
			log.Error("Unable to reset sequence %s for %s. Error: %v", newSequenceName, tableName, err)
			return err
		}
	}

	return sess.Commit()
}

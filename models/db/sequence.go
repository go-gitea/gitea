// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"
	"fmt"
	"regexp"

	"code.gitea.io/gitea/modules/setting"
)

// CountBadSequences looks for broken sequences from recreate-table mistakes
func CountBadSequences(_ context.Context) (int64, error) {
	if !setting.Database.Type.IsPostgreSQL() {
		return 0, nil
	}

	sess := xormEngine.NewSession()
	defer sess.Close()

	var sequences []string
	schema := xormEngine.Dialect().URI().Schema

	sess.Engine().SetSchema("")
	if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE 'tmp_recreate__%_id_seq%' AND sequence_catalog = ?", setting.Database.Name).Find(&sequences); err != nil {
		return 0, err
	}
	sess.Engine().SetSchema(schema)

	return int64(len(sequences)), nil
}

// FixBadSequences fixes for broken sequences from recreate-table mistakes
func FixBadSequences(_ context.Context) error {
	if !setting.Database.Type.IsPostgreSQL() {
		return nil
	}

	sess := xormEngine.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	var sequences []string
	schema := sess.Engine().Dialect().URI().Schema

	sess.Engine().SetSchema("")
	if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE 'tmp_recreate__%_id_seq%' AND sequence_catalog = ?", setting.Database.Name).Find(&sequences); err != nil {
		return err
	}
	sess.Engine().SetSchema(schema)

	sequenceRegexp := regexp.MustCompile(`tmp_recreate__(\w+)_id_seq.*`)

	for _, sequence := range sequences {
		tableName := sequenceRegexp.FindStringSubmatch(sequence)[1]
		newSequenceName := tableName + "_id_seq"
		if _, err := sess.Exec(fmt.Sprintf("ALTER SEQUENCE `%s` RENAME TO `%s`", sequence, newSequenceName)); err != nil {
			return err
		}
		if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', COALESCE((SELECT MAX(id)+1 FROM `%s`), 1), false)", newSequenceName, tableName)); err != nil {
			return err
		}
	}

	return sess.Commit()
}

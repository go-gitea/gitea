// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"testing"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm/names"
)

func Test_DropTableColumns(t *testing.T) {
	x, deferable := PrepareTestEnv(t, 0)
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	type DropTest struct {
		ID            int64 `xorm:"pk autoincr"`
		FirstColumn   string
		ToDropColumn  string `xorm:"unique"`
		AnotherColumn int64
		CreatedUnix   timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	columns := []string{
		"first_column",
		"to_drop_column",
		"another_column",
		"created_unix",
		"updated_unix",
	}

	for i := range columns {
		x.SetMapper(names.GonicMapper{})
		if err := x.Sync(new(DropTest)); err != nil {
			t.Errorf("unable to create DropTest table: %v", err)
			return
		}
		sess := x.NewSession()
		if err := sess.Begin(); err != nil {
			sess.Close()
			t.Errorf("unable to begin transaction: %v", err)
			return
		}
		if err := DropTableColumns(sess, "drop_test", columns[i:]...); err != nil {
			sess.Close()
			t.Errorf("Unable to drop columns[%d:]: %s from drop_test: %v", i, columns[i:], err)
			return
		}
		if err := sess.Commit(); err != nil {
			sess.Close()
			t.Errorf("unable to commit transaction: %v", err)
			return
		}
		sess.Close()
		if err := x.DropTables(new(DropTest)); err != nil {
			t.Errorf("unable to drop table: %v", err)
			return
		}
		for j := range columns[i+1:] {
			x.SetMapper(names.GonicMapper{})
			if err := x.Sync(new(DropTest)); err != nil {
				t.Errorf("unable to create DropTest table: %v", err)
				return
			}
			dropcols := append([]string{columns[i]}, columns[j+i+1:]...)
			sess := x.NewSession()
			if err := sess.Begin(); err != nil {
				sess.Close()
				t.Errorf("unable to begin transaction: %v", err)
				return
			}
			if err := DropTableColumns(sess, "drop_test", dropcols...); err != nil {
				sess.Close()
				t.Errorf("Unable to drop columns: %s from drop_test: %v", dropcols, err)
				return
			}
			if err := sess.Commit(); err != nil {
				sess.Close()
				t.Errorf("unable to commit transaction: %v", err)
				return
			}
			sess.Close()
			if err := x.DropTables(new(DropTest)); err != nil {
				t.Errorf("unable to drop table: %v", err)
				return
			}
		}
	}
}

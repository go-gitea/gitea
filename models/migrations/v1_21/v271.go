// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint
import (
	"code.gitea.io/gitea/modules/timeutil"

	"fmt"

	"xorm.io/xorm"
)

func AddArchivedUnixColumInLabelTable(x *xorm.Engine) error {
	type Label struct {
		ArchivedUnix timeutil.TimeStamp `xorm:"DEFAULT 0"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(Label)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}

	_, err := sess.Exec("UPDATE label SET archived_unix=0")
	if err != nil {
		return err
	}
	return sess.Commit()
}

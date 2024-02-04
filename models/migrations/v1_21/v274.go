// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint
import (
	"time"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddExpiredUnixColumnInActionArtifactTable(x *xorm.Engine) error {
	type ActionArtifact struct {
		ExpiredUnix timeutil.TimeStamp `xorm:"index"` // time when the artifact will be expired
	}
	if err := x.Sync(new(ActionArtifact)); err != nil {
		return err
	}
	return updateArtifactsExpiredUnixTo90Days(x)
}

func updateArtifactsExpiredUnixTo90Days(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}
	expiredTime := time.Now().AddDate(0, 0, 90).Unix()
	if _, err := sess.Exec(`UPDATE action_artifact SET expired_unix=? WHERE status='2' AND expired_unix is NULL`, expiredTime); err != nil {
		return err
	}

	return sess.Commit()
}

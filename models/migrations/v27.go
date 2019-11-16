// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func convertIntervalToDuration(x *xorm.Engine) (err error) {
	type Repository struct {
		ID      int64
		OwnerID int64
		Name    string
	}
	type Mirror struct {
		ID       int64       `xorm:"pk autoincr"`
		RepoID   int64       `xorm:"INDEX"`
		Repo     *Repository `xorm:"-"`
		Interval time.Duration
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	dialect := x.Dialect().DriverName()

	switch dialect {
	case "mysql":
		_, err = sess.Exec("ALTER TABLE mirror MODIFY `interval` BIGINT")
	case "postgres":
		_, err = sess.Exec("ALTER TABLE mirror ALTER COLUMN \"interval\" SET DATA TYPE bigint")
	case "mssql":
		_, err = sess.Exec("ALTER TABLE mirror ALTER COLUMN \"interval\" BIGINT")
	case "sqlite3":
	}

	if err != nil {
		return fmt.Errorf("Error changing mirror interval column type: %v", err)
	}

	var mirrors []Mirror
	err = sess.Table("mirror").Select("*").Find(&mirrors)
	if err != nil {
		return fmt.Errorf("Query repositories: %v", err)
	}
	for _, mirror := range mirrors {
		mirror.Interval *= time.Hour
		if mirror.Interval < setting.Mirror.MinInterval {
			log.Info("Mirror interval less than Mirror.MinInterval, setting default interval: repo id %v", mirror.RepoID)
			mirror.Interval = setting.Mirror.DefaultInterval
		}
		log.Debug("Mirror interval set to %v for repo id %v", mirror.Interval, mirror.RepoID)
		_, err := sess.ID(mirror.ID).Cols("interval").Update(mirror)
		if err != nil {
			return fmt.Errorf("update mirror interval failed: %v", err)
		}
	}

	return sess.Commit()
}

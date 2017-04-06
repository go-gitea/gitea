// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"time"

	"github.com/go-xorm/xorm"
)

func convertIntervalToDuration(x *xorm.Engine) (err error) {
	type Repository struct {
		ID      int64
		OwnerID int64
		Name    string
	}
	type Mirror struct {
		ID          int64       `xorm:"pk autoincr"`
		RepoID      int64       `xorm:"INDEX"`
		Repo        *Repository `xorm:"-"`
		Interval    time.Duration
		EnablePrune bool `xorm:"NOT NULL DEFAULT true"`

		Updated        time.Time `xorm:"-"`
		UpdatedUnix    int64     `xorm:"INDEX"`
		NextUpdate     time.Time `xorm:"-"`
		NextUpdateUnix int64     `xorm:"INDEX"`

		address string `xorm:"-"`
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	var mirrors []Mirror
	err = sess.Table("mirror").Select("*").Find(&mirrors)
	if err != nil {
		return fmt.Errorf("Query repositories: %v", err)
	}
	for _, mirror := range mirrors {
		mirror.Interval = mirror.Interval * time.Hour
		_, err := sess.Id(mirror.ID).Cols("interval").Update(mirror)
		if err != nil {
			return fmt.Errorf("update mirror interval failed: %v", err)
		}
	}

	return sess.Commit()
}

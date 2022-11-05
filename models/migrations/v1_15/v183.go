// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_15 //nolint

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreatePushMirrorTable(x *xorm.Engine) error {
	type PushMirror struct {
		ID         int64 `xorm:"pk autoincr"`
		RepoID     int64 `xorm:"INDEX"`
		RemoteName string

		Interval       time.Duration
		CreatedUnix    timeutil.TimeStamp `xorm:"created"`
		LastUpdateUnix timeutil.TimeStamp `xorm:"INDEX last_update"`
		LastError      string             `xorm:"text"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(PushMirror)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}

	return sess.Commit()
}

// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addNotificationBy(x *xorm.Engine) error {
	// NotificationBy is the reason why user is notified.
	type NotificationBy uint8

	const (
		// NotificationByWatchRepo reprensents notification according to
		NotificationByWatchRepo NotificationBy = iota + 1
		// NotificationBySubcribeIssue
		NotificationBySubcribeIssue
		// NotificationByMentioned
		NotificationByMentioned
		// NotificationByParticipated
		NotificationByParticipated
	)

	type Notification struct {
		By NotificationBy `xorm:"SMALLINT INDEX NOT NULL"`
	}

	if err := x.Sync2(new(Notification)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE notification SET by=?", NotificationByWatchRepo)
	return err
}

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddAuditEventTable(x *xorm.Engine) error {
	type AuditEvent struct {
		ID          int64  `xorm:"pk autoincr"`
		Action      string `xorm:"INDEX NOT NULL"`
		ActorID     int64  `xorm:"INDEX NOT NULL"`
		ScopeType   string `xorm:"INDEX(scope) NOT NULL"`
		ScopeID     int64  `xorm:"INDEX(scope) NOT NULL"`
		TargetType  string `xorm:"NOT NULL"`
		TargetID    int64  `xorm:"NOT NULL"`
		Message     string
		IPAddress   string
		CreatedUnix timeutil.TimeStamp `xorm:"created INDEX NOT NULL"`
	}

	return x.Sync(&AuditEvent{})
}

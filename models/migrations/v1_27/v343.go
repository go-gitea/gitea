// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

type AuditEvent struct {
	ID            int64  `xorm:"pk autoincr"`
	Action        string `xorm:"INDEX NOT NULL"`
	ActorID       int64  `xorm:"INDEX NOT NULL"`
	ActorName     string
	ScopeType     string `xorm:"INDEX(scope) NOT NULL"`
	ScopeID       int64  `xorm:"INDEX(scope) NOT NULL"`
	ScopeName     string
	Message       string
	Metadata      string `xorm:"TEXT JSON"`
	IPAddress     string
	TimestampUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL"`
}

func (*AuditEvent) TableName() string {
	return "audit_event"
}

func AddAuditEventTable(x db.EngineMigration) error {
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(AuditEvent))
	return err
}

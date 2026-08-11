// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

type AuditEvent struct {
	ID            int64  `xorm:"pk autoincr"`
	Action        string `xorm:"INDEX NOT NULL"`
	ActorID       int64  `xorm:"INDEX NOT NULL"`
	ActorName     string
	ScopeID       int64  `xorm:"INDEX(scope) NOT NULL"`
	ScopeType     string `xorm:"INDEX INDEX(scope) NOT NULL"`
	ScopeName     string
	Origin        string `xorm:"INDEX NOT NULL"`
	Message       string
	Metadata      string `xorm:"LONGTEXT JSON"`
	IPAddress     string
	TimestampUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL"`
}

func (*AuditEvent) TableName() string {
	return "audit_event"
}

func AddAuditEventTable(x base.EngineMigration) error {
	return x.Sync(new(AuditEvent))
}

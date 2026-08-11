// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Event))
}

type Event struct {
	ID            int64  `xorm:"pk autoincr"`
	Action        Action `xorm:"INDEX NOT NULL"`
	ActorID       int64  `xorm:"INDEX NOT NULL"`
	ActorName     string
	ScopeID       int64     `xorm:"INDEX(scope) NOT NULL"` // Entity ID within ScopeType; zero for system.
	ScopeType     ScopeType `xorm:"INDEX INDEX(scope) NOT NULL"`
	ScopeName     string
	Origin        Origin `xorm:"INDEX NOT NULL"`
	Message       string
	Metadata      string `xorm:"LONGTEXT JSON"`
	IPAddress     string
	TimestampUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL"`
}

func (*Event) TableName() string {
	return "audit_event"
}

func InsertEvent(ctx context.Context, e *Event) (*Event, error) {
	return e, db.Insert(ctx, e)
}

type EventSort = string

const (
	SortTimestampAsc  EventSort = "timestamp_asc"
	SortTimestampDesc EventSort = "timestamp_desc"
)

type EventSearchOptions struct {
	Action    Action
	ActorID   int64
	ScopeType ScopeType
	ScopeID   int64
	Origin    Origin
	Sort      EventSort
	db.Paginator
}

func (opts *EventSearchOptions) ToConds() builder.Cond {
	cond := builder.NewCond()

	if opts.Action != "" {
		cond = cond.And(builder.Eq{"action": opts.Action})
	}
	if opts.ActorID != 0 {
		cond = cond.And(builder.Eq{"actor_id": opts.ActorID})
	}
	// applied independently so a missing scope ID narrows the query instead of
	// silently widening it to every scope
	if opts.ScopeType != "" {
		cond = cond.And(builder.Eq{"audit_event.scope_type": opts.ScopeType})
	}
	if opts.ScopeID != 0 {
		cond = cond.And(builder.Eq{"audit_event.scope_id": opts.ScopeID})
	}
	if opts.Origin != "" {
		cond = cond.And(builder.Eq{"audit_event.origin": opts.Origin})
	}

	return cond
}

func (opts *EventSearchOptions) configureOrderBy(e db.Engine) {
	switch opts.Sort {
	case SortTimestampAsc:
		e.Asc("timestamp_unix", "id")
	default:
		e.Desc("timestamp_unix", "id")
	}
}

func FindEvents(ctx context.Context, opts *EventSearchOptions) ([]*Event, int64, error) {
	sess := db.GetEngine(ctx).
		Where(opts.ToConds()).
		Table("audit_event")

	opts.configureOrderBy(sess)

	if opts.Paginator != nil {
		db.SetSessionPagination(sess, opts)
	}

	evs := make([]*Event, 0, 10)
	count, err := sess.FindAndCount(&evs)
	return evs, count, err
}

// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Event))
}

type Event struct {
	ID            int64      `xorm:"pk autoincr"`
	Action        Action     `xorm:"INDEX NOT NULL"`
	ActorID       int64      `xorm:"INDEX NOT NULL"`
	ScopeType     ObjectType `xorm:"INDEX(scope) NOT NULL"`
	ScopeID       int64      `xorm:"INDEX(scope) NOT NULL"`
	TargetType    ObjectType `xorm:"NOT NULL"`
	TargetID      int64      `xorm:"NOT NULL"`
	Message       string
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
	ScopeType ObjectType
	ScopeID   int64
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
	if opts.ScopeID != 0 && opts.ScopeType != "" {
		cond = cond.And(builder.Eq{
			"audit_event.scope_type": opts.ScopeType,
			"audit_event.scope_id":   opts.ScopeID,
		})
	}

	return cond
}

func (opts *EventSearchOptions) configureOrderBy(e db.Engine) {
	switch opts.Sort {
	case SortTimestampAsc:
		e.Asc("timestamp_unix")
	default:
		e.Desc("timestamp_unix")
	}

	// Sort by id for stable order with duplicates in the other field
	e.Asc("id")
}

func FindEvents(ctx context.Context, opts *EventSearchOptions) ([]*Event, int64, error) {
	sess := db.GetEngine(ctx).
		Where(opts.ToConds()).
		Table("audit_event")

	opts.configureOrderBy(sess)

	if opts.Paginator != nil {
		sess = db.SetSessionPagination(sess, opts)
	}

	evs := make([]*Event, 0, 10)
	count, err := sess.FindAndCount(&evs)
	return evs, count, err
}

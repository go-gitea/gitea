// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

func init() {
	db.RegisterModel(new(Event))
}

type Event struct {
	ID               int64  `xorm:"pk autoincr"`
	Action           Action `xorm:"INDEX NOT NULL"`
	ActorID          int64  `xorm:"INDEX NOT NULL"`
	ActorName        string
	ImpersonatorID   int64 `xorm:"INDEX"` // Admin acting as the actor; zero when the actor acted themselves.
	ImpersonatorName string
	ScopeID          int64     `xorm:"INDEX(scope) NOT NULL"` // Entity ID within ScopeType; zero for system.
	ScopeType        ScopeType `xorm:"INDEX INDEX(scope) NOT NULL"`
	ScopeName        string
	Origin           Origin `xorm:"INDEX NOT NULL"`
	Message          string
	Metadata         string `xorm:"LONGTEXT JSON"`
	IPAddress        string
	TimestampUnix    timeutil.TimeStamp `xorm:"INDEX NOT NULL"`
}

func (*Event) TableName() string {
	return "audit_event"
}

func (e *Event) Actor() EntityRef {
	return EntityRef{Type: ScopeUser, ID: e.ActorID, Name: e.ActorName}
}

// Impersonator returns the admin who acted as the actor, or nil.
func (e *Event) Impersonator() *EntityRef {
	if e.ImpersonatorID == 0 && e.ImpersonatorName == "" {
		return nil
	}
	return &EntityRef{Type: ScopeUser, ID: e.ImpersonatorID, Name: e.ImpersonatorName}
}

func (e *Event) Scope() EntityRef {
	return EntityRef{Type: e.ScopeType, ID: e.ScopeID, Name: e.ScopeName}
}

func (e *Event) Time() time.Time {
	return e.TimestampUnix.AsTime()
}

// eventJSON is the nested JSONL export / import shape.
type eventJSON struct {
	Action       Action         `json:"action"`
	Actor        EntityRef      `json:"actor"`
	Impersonator *EntityRef     `json:"impersonator,omitempty"`
	Scope        EntityRef      `json:"scope"`
	Message      string         `json:"message"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Time         time.Time      `json:"time"`
	IPAddress    string         `json:"ip_address"`
	Origin       Origin         `json:"origin"`
}

func (e *Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(eventJSON{
		Action:       e.Action,
		Actor:        e.Actor(),
		Impersonator: e.Impersonator(),
		Scope:        e.Scope(),
		Message:      e.Message,
		Metadata:     DecodeMetadata(e.Metadata),
		Time:         e.Time(),
		IPAddress:    e.IPAddress,
		Origin:       e.Origin,
	})
}

func (e *Event) UnmarshalJSON(data []byte) error {
	var j eventJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	e.Action = j.Action
	e.ActorID = j.Actor.ID
	e.ActorName = j.Actor.Name
	if j.Impersonator != nil {
		e.ImpersonatorID = j.Impersonator.ID
		e.ImpersonatorName = j.Impersonator.Name
	}
	e.ScopeType = j.Scope.Type
	e.ScopeID = j.Scope.ID
	e.ScopeName = j.Scope.Name
	e.Message = j.Message
	e.Metadata = EncodeMetadata(j.Metadata)
	e.IPAddress = j.IPAddress
	e.Origin = j.Origin
	e.TimestampUnix = timeutil.TimeStamp(j.Time.Unix())
	return nil
}

func EncodeMetadata(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Error("Failed to encode audit metadata: %v", err)
		return ""
	}
	return string(b)
}

func DecodeMetadata(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		log.Error("Failed to decode audit metadata: %v", err)
		return nil
	}
	return m
}

func InsertEvent(ctx context.Context, e *Event) error {
	return db.Insert(ctx, e)
}

// DeleteOldEvents removes events older than the given duration, keeping everything if it is not positive.
func DeleteOldEvents(ctx context.Context, olderThan time.Duration) error {
	if olderThan <= 0 {
		return nil
	}
	_, err := db.GetEngine(ctx).Where("timestamp_unix < ?", time.Now().Add(-olderThan).Unix()).Delete(&Event{})
	return err
}

type EventSort string

const (
	SortTimestampAsc  EventSort = "timestamp_asc"
	SortTimestampDesc EventSort = "timestamp_desc"
)

type EventSearchOptions struct {
	db.ListOptions
	Action    Action
	ActorID   int64
	ScopeType ScopeType
	ScopeID   int64
	Origin    Origin
	Sort      EventSort
}

func (opts *EventSearchOptions) ToConds() builder.Cond {
	cond := builder.NewCond()

	if opts.Action != "" {
		cond = cond.And(builder.Eq{"action": opts.Action})
	}
	if opts.ActorID != 0 {
		// an impersonated event belongs to both the actor and the admin behind it
		cond = cond.And(builder.Eq{"actor_id": opts.ActorID}.Or(builder.Eq{"impersonator_id": opts.ActorID}))
	}
	// applied independently so a missing scope ID narrows the query instead of
	// silently widening it to every scope
	if opts.ScopeType != "" {
		cond = cond.And(builder.Eq{"scope_type": opts.ScopeType})
	}
	if opts.ScopeID != 0 {
		cond = cond.And(builder.Eq{"scope_id": opts.ScopeID})
	}
	if opts.Origin != "" {
		cond = cond.And(builder.Eq{"origin": opts.Origin})
	}

	return cond
}

func (opts *EventSearchOptions) ToOrders() string {
	if opts.Sort == SortTimestampAsc {
		return "timestamp_unix ASC, id ASC"
	}
	return "timestamp_unix DESC, id DESC"
}

func FindEvents(ctx context.Context, opts *EventSearchOptions) ([]*Event, int64, error) {
	return db.FindAndCount[Event](ctx, opts)
}

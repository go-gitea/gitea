// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
	"reflect"
	"time"
	"unsafe"

	ap "github.com/go-ap/activitypub"
	"github.com/valyala/fastjson"
)

const (
	TicketType ap.ActivityVocabularyType = "Ticket"
)

type Ticket struct {
	ap.Object
	// Dependants Collection of Tickets which depend on this ticket
	Dependants ap.ItemCollection `jsonld:"dependants,omitempty"`
	// Dependencies Collection of Tickets on which this ticket depends
	Dependencies ap.ItemCollection `jsonld:"dependencies,omitempty"`
	// IsResolved Whether the work on this ticket is done
	IsResolved bool `jsonld:"isResolved,omitempty"`
	// ResolvedBy If the work on this ticket is done, who marked the ticket as resolved, or which activity did so
	ResolvedBy ap.Item `jsonld:"resolvedBy,omitempty"`
	// Resolved When the ticket has been marked as resolved
	Resolved time.Time `jsonld:"resolved,omitempty"`
	// Origin The head branch if this ticket is a pull request
	Origin ap.Item `jsonld:"origin,omitempty"`
	// Target The base branch if this ticket is a pull request
	Target ap.Item `jsonld:"target,omitempty"`
}

// TicketNew initializes a Ticket type Object
func TicketNew() *Ticket {
	a := ap.ObjectNew(TicketType)
	o := Ticket{Object: *a}
	return &o
}

func (t Ticket) MarshalJSON() ([]byte, error) {
	b, err := t.Object.MarshalJSON()
	if len(b) == 0 || err != nil {
		return nil, err
	}

	b = b[:len(b)-1]
	if t.Dependants != nil {
		ap.JSONWriteItemCollectionJSONProp(&b, "dependants", t.Dependants)
	}
	if t.Dependencies != nil {
		ap.JSONWriteItemCollectionJSONProp(&b, "dependencies", t.Dependencies)
	}
	ap.JSONWriteBoolJSONProp(&b, "isResolved", t.IsResolved)
	if t.ResolvedBy != nil {
		ap.JSONWriteItemJSONProp(&b, "resolvedBy", t.ResolvedBy)
	}
	if !t.Resolved.IsZero() {
		ap.JSONWriteTimeJSONProp(&b, "resolved", t.Resolved)
	}
	if t.Origin != nil {
		ap.JSONWriteItemJSONProp(&b, "origin", t.Origin)
	}
	if t.Target != nil {
		ap.JSONWriteItemJSONProp(&b, "target", t.Target)
	}
	ap.JSONWrite(&b, '}')
	return b, nil
}

func JSONLoadTicket(val *fastjson.Value, t *Ticket) error {
	if err := ap.OnObject(&t.Object, func(o *ap.Object) error {
		return ap.JSONLoadObject(val, o)
	}); err != nil {
		return err
	}

	t.Dependants = ap.JSONGetItems(val, "dependants")
	t.Dependencies = ap.JSONGetItems(val, "dependencies")
	t.IsResolved = ap.JSONGetBoolean(val, "isResolved")
	t.ResolvedBy = ap.JSONGetItem(val, "resolvedBy")
	t.Resolved = ap.JSONGetTime(val, "resolved")
	t.Origin = ap.JSONGetItem(val, "origin")
	t.Target = ap.JSONGetItem(val, "target")
	return nil
}

func (t *Ticket) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}
	return JSONLoadTicket(val, t)
}

// ToTicket tries to convert the it Item to a Ticket object.
func ToTicket(it ap.Item) (*Ticket, error) {
	switch i := it.(type) {
	case *Ticket:
		return i, nil
	case Ticket:
		return &i, nil
	case *ap.Object:
		return (*Ticket)(unsafe.Pointer(i)), nil
	case ap.Object:
		return (*Ticket)(unsafe.Pointer(&i)), nil
	default:
		// NOTE(marius): this is an ugly way of dealing with the interface conversion error: types from different scopes
		typ := reflect.TypeOf(new(Ticket))
		if i, ok := reflect.ValueOf(it).Convert(typ).Interface().(*Ticket); ok {
			return i, nil
		}
	}
	return nil, ap.ErrorInvalidType[ap.Object](it)
}

type withTicketFn func(*Ticket) error

// OnTicket calls function fn on it Item if it can be asserted to type *Ticket
func OnTicket(it ap.Item, fn withTicketFn) error {
	if it == nil {
		return nil
	}
	ob, err := ToTicket(it)
	if err != nil {
		return err
	}
	return fn(ob)
}

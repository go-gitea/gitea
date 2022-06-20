// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
	"time"

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
		return make([]byte, 0), err
	}

	b = b[:len(b)-1]
	if t.Dependants != nil {
		ap.WriteItemCollectionJSONProp(&b, "dependants", t.Dependants)
	}
	if t.Dependencies != nil {
		ap.WriteItemCollectionJSONProp(&b, "dependencies", t.Dependencies)
	}
	ap.WriteBoolJSONProp(&b, "isResolved", t.IsResolved)
	if t.ResolvedBy != nil {
		ap.WriteItemJSONProp(&b, "resolvedBy", t.ResolvedBy)
	}
	if !t.Resolved.IsZero() {
		ap.WriteTimeJSONProp(&b, "resolved", t.Resolved)
	}
	ap.Write(&b, '}')
	return b, nil
}

func (t *Ticket) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}

	t.Dependants = ap.JSONGetItems(val, "dependants")
	t.Dependencies = ap.JSONGetItems(val, "dependencies")
	t.IsResolved = ap.JSONGetBoolean(val, "isResolved")
	t.ResolvedBy = ap.JSONGetItem(val, "resolvedBy")
	t.Resolved = ap.JSONGetTime(val, "resolved")

	return ap.OnObject(&t.Object, func(a *ap.Object) error {
		return ap.LoadObject(val, a)
	})
}

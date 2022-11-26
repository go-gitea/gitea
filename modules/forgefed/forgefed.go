// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
	ap "github.com/go-ap/activitypub"
	"github.com/valyala/fastjson"
)

const ForgeFedNamespaceURI = "https://forgefed.org/ns"

// GetItemByType instantiates a new ForgeFed object if the type matches
// otherwise it defaults to existing activitypub package typer function.
func GetItemByType(typ ap.ActivityVocabularyType) (ap.Item, error) {
	switch typ {
	case CommitType:
		return CommitNew(), nil
	case BranchType:
		return BranchNew(), nil
	case RepositoryType:
		return RepositoryNew(""), nil
	case PushType:
		return PushNew(), nil
	case TicketType:
		return TicketNew(), nil
	}
	return ap.GetItemByType(typ)
}

// JSONUnmarshalerFn is the function that will load the data from a fastjson.Value into an Item
// that the go-ap/activitypub package doesn't know about.
func JSONUnmarshalerFn(typ ap.ActivityVocabularyType, val *fastjson.Value, i ap.Item) error {
	switch typ {
	case CommitType:
		return OnCommit(i, func(c *Commit) error {
			return JSONLoadCommit(val, c)
		})
	case BranchType:
		return OnBranch(i, func(b *Branch) error {
			return JSONLoadBranch(val, b)
		})
	case RepositoryType:
		return OnRepository(i, func(r *Repository) error {
			return JSONLoadRepository(val, r)
		})
	case PushType:
		return OnPush(i, func(p *Push) error {
			return JSONLoadPush(val, p)
		})
	case TicketType:
		return OnTicket(i, func(t *Ticket) error {
			return JSONLoadTicket(val, t)
		})
	}
	return nil
}

// NotEmpty is the function that checks if an object is empty
func NotEmpty(i ap.Item) bool {
	if ap.IsNil(i) {
		return false
	}
	var notEmpty bool
	switch i.GetType() {
	case CommitType:
		OnCommit(i, func(c *Commit) error {
			notEmpty = ap.NotEmpty(c.Object)
			return nil
		})
	case BranchType:
		OnBranch(i, func(b *Branch) error {
			notEmpty = ap.NotEmpty(b.Object)
			return nil
		})
	case RepositoryType:
		OnRepository(i, func(r *Repository) error {
			notEmpty = ap.NotEmpty(r.Actor)
			return nil
		})
	case PushType:
		OnPush(i, func(p *Push) error {
			notEmpty = ap.NotEmpty(p.Object)
			return nil
		})
	case TicketType:
		OnTicket(i, func(t *Ticket) error {
			notEmpty = ap.NotEmpty(t.Object)
			return nil
		})
	default:
		notEmpty = ap.NotEmpty(i)
	}
	return notEmpty
}

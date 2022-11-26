// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
	"reflect"
	"unsafe"

	ap "github.com/go-ap/activitypub"
	"github.com/valyala/fastjson"
)

const (
	RepositoryType ap.ActivityVocabularyType = "Repository"
)

type Repository struct {
	ap.Actor
	// Team Collection of actors who have management/push access to the repository
	Team ap.Item `jsonld:"team,omitempty"`
	// Forks OrderedCollection of repositories that are forks of this repository
	Forks ap.Item `jsonld:"forks,omitempty"`
	// ForkedFrom Identifies the repository which this repository was created as a fork
	ForkedFrom ap.Item `jsonld:"forkedFrom,omitempty"`
}

// RepositoryNew initializes a Repository type actor
func RepositoryNew(id ap.ID) *Repository {
	a := ap.ActorNew(id, RepositoryType)
	o := Repository{Actor: *a}
	return &o
}

func (r Repository) MarshalJSON() ([]byte, error) {
	b, err := r.Actor.MarshalJSON()
	if len(b) == 0 || err != nil {
		return nil, err
	}

	b = b[:len(b)-1]
	if r.Team != nil {
		ap.JSONWriteItemJSONProp(&b, "team", r.Team)
	}
	if r.Forks != nil {
		ap.JSONWriteItemJSONProp(&b, "forks", r.Forks)
	}
	if r.ForkedFrom != nil {
		ap.JSONWriteItemJSONProp(&b, "forkedFrom", r.ForkedFrom)
	}
	ap.JSONWrite(&b, '}')
	return b, nil
}

func JSONLoadRepository(val *fastjson.Value, r *Repository) error {
	if err := ap.OnActor(&r.Actor, func(a *ap.Actor) error {
		return ap.JSONLoadActor(val, a)
	}); err != nil {
		return err
	}

	r.Team = ap.JSONGetItem(val, "team")
	r.Forks = ap.JSONGetItem(val, "forks")
	r.ForkedFrom = ap.JSONGetItem(val, "forkedFrom")
	return nil
}

func (r *Repository) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}
	return JSONLoadRepository(val, r)
}

// ToRepository tries to convert the it Item to a Repository Actor.
func ToRepository(it ap.Item) (*Repository, error) {
	switch i := it.(type) {
	case *Repository:
		return i, nil
	case Repository:
		return &i, nil
	case *ap.Actor:
		return (*Repository)(unsafe.Pointer(i)), nil
	case ap.Actor:
		return (*Repository)(unsafe.Pointer(&i)), nil
	default:
		// NOTE(marius): this is an ugly way of dealing with the interface conversion error: types from different scopes
		typ := reflect.TypeOf(new(Repository))
		if i, ok := reflect.ValueOf(it).Convert(typ).Interface().(*Repository); ok {
			return i, nil
		}
	}
	return nil, ap.ErrorInvalidType[ap.Actor](it)
}

type withRepositoryFn func(*Repository) error

// OnRepository calls function fn on it Item if it can be asserted to type *Repository
func OnRepository(it ap.Item, fn withRepositoryFn) error {
	if it == nil {
		return nil
	}
	ob, err := ToRepository(it)
	if err != nil {
		return err
	}
	return fn(ob)
}

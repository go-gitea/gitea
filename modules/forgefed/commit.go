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
	CommitType ap.ActivityVocabularyType = "Commit"
)

type Commit struct {
	ap.Object
	// Created time at which the commit was written by its author
	Created time.Time `jsonld:"created,omitempty"`
	// Committed time at which the commit was committed by its committer
	Committed time.Time `jsonld:"committed,omitempty"`
}

// CommitNew initializes a Commit type Object
func CommitNew() *Commit {
	a := ap.ObjectNew(CommitType)
	o := Commit{Object: *a}
	return &o
}

func (c Commit) MarshalJSON() ([]byte, error) {
	b, err := c.Object.MarshalJSON()
	if len(b) == 0 || err != nil {
		return nil, err
	}

	b = b[:len(b)-1]
	if !c.Created.IsZero() {
		ap.JSONWriteTimeJSONProp(&b, "created", c.Created)
	}
	if !c.Committed.IsZero() {
		ap.JSONWriteTimeJSONProp(&b, "committed", c.Committed)
	}
	ap.JSONWrite(&b, '}')
	return b, nil
}

func JSONLoadCommit(val *fastjson.Value, c *Commit) error {
	if err := ap.OnObject(&c.Object, func(o *ap.Object) error {
		return ap.JSONLoadObject(val, o)
	}); err != nil {
		return err
	}

	c.Created = ap.JSONGetTime(val, "created")
	c.Committed = ap.JSONGetTime(val, "committed")
	return nil
}

func (c *Commit) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}
	return JSONLoadCommit(val, c)
}

// ToCommit tries to convert the it Item to a Commit object.
func ToCommit(it ap.Item) (*Commit, error) {
	switch i := it.(type) {
	case *Commit:
		return i, nil
	case Commit:
		return &i, nil
	case *ap.Object:
		return (*Commit)(unsafe.Pointer(i)), nil
	case ap.Object:
		return (*Commit)(unsafe.Pointer(&i)), nil
	default:
		// NOTE(marius): this is an ugly way of dealing with the interface conversion error: types from different scopes
		typ := reflect.TypeOf(new(Commit))
		if i, ok := reflect.ValueOf(it).Convert(typ).Interface().(*Commit); ok {
			return i, nil
		}
	}
	return nil, ap.ErrorInvalidType[ap.Object](it)
}

type withCommitFn func(*Commit) error

// OnCommit calls function fn on it Item if it can be asserted to type *Commit
func OnCommit(it ap.Item, fn withCommitFn) error {
	if it == nil {
		return nil
	}
	ob, err := ToCommit(it)
	if err != nil {
		return err
	}
	return fn(ob)
}

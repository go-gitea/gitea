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
		return make([]byte, 0), err
	}

	b = b[:len(b)-1]
	if !c.Created.IsZero() {
		ap.WriteTimeJSONProp(&b, "created", c.Created)
	}
	if !c.Committed.IsZero() {
		ap.WriteTimeJSONProp(&b, "committed", c.Committed)
	}
	ap.Write(&b, '}')
	return b, nil
}

func (c *Commit) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}

	c.Created = ap.JSONGetTime(val, "created")
	c.Committed = ap.JSONGetTime(val, "committed")

	return ap.OnObject(&c.Object, func(a *ap.Object) error {
		return ap.LoadObject(val, a)
	})
}

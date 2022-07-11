// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
	ap "gitea.com/Ta180m/activitypub"
	"github.com/valyala/fastjson"
)

const (
	PushType ap.ActivityVocabularyType = "Push"
)

type Push struct {
	ap.Object
	// Target the specific repo history tip onto which the commits were added
	Target ap.Item `jsonld:"target,omitempty"`
	// HashBefore hash before adding the new commits
	HashBefore ap.Item `jsonld:"hashBefore,omitempty"`
	// HashAfter hash before adding the new commits
	HashAfter ap.Item `jsonld:"hashAfter,omitempty"`
}

// PushNew initializes a Push type Object
func PushNew() *Push {
	a := ap.ObjectNew(PushType)
	o := Push{Object: *a}
	return &o
}

func (p Push) MarshalJSON() ([]byte, error) {
	b, err := p.Object.MarshalJSON()
	if len(b) == 0 || err != nil {
		return make([]byte, 0), err
	}

	b = b[:len(b)-1]
	if p.Target != nil {
		ap.WriteItemJSONProp(&b, "target", p.Target)
	}
	if p.HashBefore != nil {
		ap.WriteItemJSONProp(&b, "hashBefore", p.HashBefore)
	}
	if p.HashAfter != nil {
		ap.WriteItemJSONProp(&b, "hashAfter", p.HashAfter)
	}
	ap.Write(&b, '}')
	return b, nil
}

func (c *Push) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}

	c.Target = ap.JSONGetItem(val, "target")
	c.HashBefore = ap.JSONGetItem(val, "hashBefore")
	c.HashAfter = ap.JSONGetItem(val, "hashAfter")

	return ap.OnObject(&c.Object, func(a *ap.Object) error {
		return ap.LoadObject(val, a)
	})
}

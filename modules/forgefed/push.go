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
		return nil, err
	}

	b = b[:len(b)-1]
	if p.Target != nil {
		ap.JSONWriteItemJSONProp(&b, "target", p.Target)
	}
	if p.HashBefore != nil {
		ap.JSONWriteItemJSONProp(&b, "hashBefore", p.HashBefore)
	}
	if p.HashAfter != nil {
		ap.JSONWriteItemJSONProp(&b, "hashAfter", p.HashAfter)
	}
	ap.JSONWrite(&b, '}')
	return b, nil
}

func JSONLoadPush(val *fastjson.Value, p *Push) error {
	if err := ap.OnObject(&p.Object, func(o *ap.Object) error {
		return ap.JSONLoadObject(val, o)
	}); err != nil {
		return err
	}

	p.Target = ap.JSONGetItem(val, "target")
	p.HashBefore = ap.JSONGetItem(val, "hashBefore")
	p.HashAfter = ap.JSONGetItem(val, "hashAfter")
	return nil
}

func (p *Push) UnmarshalJSON(data []byte) error {
	par := fastjson.Parser{}
	val, err := par.ParseBytes(data)
	if err != nil {
		return err
	}
	return JSONLoadPush(val, p)
}

// ToPush tries to convert the it Item to a Push object.
func ToPush(it ap.Item) (*Push, error) {
	switch i := it.(type) {
	case *Push:
		return i, nil
	case Push:
		return &i, nil
	case *ap.Object:
		return (*Push)(unsafe.Pointer(i)), nil
	case ap.Object:
		return (*Push)(unsafe.Pointer(&i)), nil
	default:
		// NOTE(marius): this is an ugly way of dealing with the interface conversion error: types from different scopes
		typ := reflect.TypeOf(new(Push))
		if i, ok := reflect.ValueOf(it).Convert(typ).Interface().(*Push); ok {
			return i, nil
		}
	}
	return nil, ap.ErrorInvalidType[ap.Object](it)
}

type withPushFn func(*Push) error

// OnPush calls function fn on it Item if it can be asserted to type *Push
func OnPush(it ap.Item, fn withPushFn) error {
	if it == nil {
		return nil
	}
	ob, err := ToPush(it)
	if err != nil {
		return err
	}
	return fn(ob)
}

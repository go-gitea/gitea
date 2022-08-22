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
	BranchType ap.ActivityVocabularyType = "Branch"
)

type Branch struct {
	ap.Object
	// Ref the unique identifier of the branch within the repo
	Ref ap.Item `jsonld:"ref,omitempty"`
}

// BranchNew initializes a Branch type Object
func BranchNew() *Branch {
	a := ap.ObjectNew(BranchType)
	o := Branch{Object: *a}
	return &o
}

func (b Branch) MarshalJSON() ([]byte, error) {
	bin, err := b.Object.MarshalJSON()
	if len(bin) == 0 || err != nil {
		return nil, err
	}

	bin = bin[:len(bin)-1]
	if b.Ref != nil {
		ap.JSONWriteItemJSONProp(&bin, "ref", b.Ref)
	}
	ap.JSONWrite(&bin, '}')
	return bin, nil
}

func JSONLoadBranch(val *fastjson.Value, b *Branch) error {
	if err := ap.OnObject(&b.Object, func(o *ap.Object) error {
		return ap.JSONLoadObject(val, o)
	}); err != nil {
		return err
	}

	b.Ref = ap.JSONGetItem(val, "ref")
	return nil
}

func (b *Branch) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}
	return JSONLoadBranch(val, b)
}

// ToBranch tries to convert the it Item to a Branch object.
func ToBranch(it ap.Item) (*Branch, error) {
	switch i := it.(type) {
	case *Branch:
		return i, nil
	case Branch:
		return &i, nil
	case *ap.Object:
		return (*Branch)(unsafe.Pointer(i)), nil
	case ap.Object:
		return (*Branch)(unsafe.Pointer(&i)), nil
	default:
		// NOTE(marius): this is an ugly way of dealing with the interface conversion error: types from different scopes
		typ := reflect.TypeOf(new(Branch))
		if i, ok := reflect.ValueOf(it).Convert(typ).Interface().(*Branch); ok {
			return i, nil
		}
	}
	return nil, ap.ErrorInvalidType[ap.Object](it)
}

type withBranchFn func(*Branch) error

// OnBranch calls function fn on it Item if it can be asserted to type *Branch
func OnBranch(it ap.Item, fn withBranchFn) error {
	if it == nil {
		return nil
	}
	ob, err := ToBranch(it)
	if err != nil {
		return err
	}
	return fn(ob)
}

// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
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

func (br Branch) MarshalJSON() ([]byte, error) {
	b, err := br.Object.MarshalJSON()
	if len(b) == 0 || err != nil {
		return make([]byte, 0), err
	}

	b = b[:len(b)-1]
	if br.Ref != nil {
		ap.WriteItemJSONProp(&b, "ref", br.Ref)
	}
	ap.Write(&b, '}')
	return b, nil
}

func (br *Branch) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}

	br.Ref = ap.JSONGetItem(val, "ref")
	
	return ap.OnObject(&br.Object, func(a *ap.Object) error {
		return ap.LoadObject(val, a)
	})
}

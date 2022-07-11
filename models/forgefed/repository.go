// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forgefed

import (
	ap "gitea.com/Ta180m/activitypub"
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
}

// RepositoryNew initializes a Repository type actor
func RepositoryNew(id ap.ID) *Repository {
	a := ap.ActorNew(id, RepositoryType)
	o := Repository{Actor: *a}
	o.Type = RepositoryType
	return &o
}

func (r Repository) MarshalJSON() ([]byte, error) {
	b, err := r.Actor.MarshalJSON()
	if len(b) == 0 || err != nil {
		return make([]byte, 0), err
	}

	b = b[:len(b)-1]
	if r.Team != nil {
		ap.WriteItemJSONProp(&b, "team", r.Team)
	}
	if r.Forks != nil {
		ap.WriteItemJSONProp(&b, "forks", r.Forks)
	}
	ap.Write(&b, '}')
	return b, nil
}

func (r *Repository) UnmarshalJSON(data []byte) error {
	p := fastjson.Parser{}
	val, err := p.ParseBytes(data)
	if err != nil {
		return err
	}

	r.Team = ap.JSONGetItem(val, "team")
	r.Forks = ap.JSONGetItem(val, "forks")

	return ap.OnActor(&r.Actor, func(a *ap.Actor) error {
		return ap.LoadActor(val, a)
	})
}

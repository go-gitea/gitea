// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	issues_model "code.gitea.io/gitea/models/issues"

	ap "github.com/go-ap/activitypub"
)

func Note(comment *issues_model.Comment) ap.Note {
	note := ap.Note{
		Type:         ap.NoteType,
		AttributedTo: ap.IRI(comment.Poster.GetIRI()),
		Context:      ap.IRI(comment.Issue.GetIRI()),
	}
	note.Content = ap.NaturalLanguageValuesNew()
	_ = note.Content.Set("en", ap.Content(comment.Content))
	return note
}

// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"strconv"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/forgefed"

	ap "github.com/go-ap/activitypub"
)

// Construct a Note object from a comment
func Note(comment *issues_model.Comment) *ap.Note {
	note := ap.Note{
		Type:         ap.NoteType,
		AttributedTo: ap.IRI(comment.Poster.GetIRI()),
		Context:      ap.IRI(comment.Issue.GetIRI()),
	}
	note.Content = ap.NaturalLanguageValuesNew()
	_ = note.Content.Set("en", ap.Content(comment.Content))
	return &note
}

// Construct a Ticket object from an issue
func Ticket(issue *issues_model.Issue) (*forgefed.Ticket, error) {
	iri := issue.GetIRI()
	ticket := forgefed.TicketNew()
	ticket.Type = forgefed.TicketType
	ticket.ID = ap.IRI(iri)

	// Setting a NaturalLanguageValue to a number causes go-ap's JSON parsing to do weird things
	// Workaround: set it to #1 instead of 1
	ticket.Name = ap.NaturalLanguageValuesNew()
	err := ticket.Name.Set("en", ap.Content("#"+strconv.FormatInt(issue.Index, 10)))
	if err != nil {
		return nil, err
	}

	err = issue.LoadRepo(db.DefaultContext)
	if err != nil {
		return nil, err
	}
	ticket.Context = ap.IRI(issue.Repo.GetIRI())

	err = issue.LoadPoster()
	if err != nil {
		return nil, err
	}
	ticket.AttributedTo = ap.IRI(issue.Poster.GetIRI())

	ticket.Summary = ap.NaturalLanguageValuesNew()
	err = ticket.Summary.Set("en", ap.Content(issue.Title))
	if err != nil {
		return nil, err
	}

	ticket.Content = ap.NaturalLanguageValuesNew()
	err = ticket.Content.Set("en", ap.Content(issue.Content))
	if err != nil {
		return nil, err
	}

	if issue.IsClosed {
		ticket.IsResolved = true
	}
	return ticket, nil
}

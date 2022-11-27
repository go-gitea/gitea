// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

// Repo function returns the Repository actor of a repo
func Repo(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/repo/{username}/{reponame} activitypub activitypubRepo
	// ---
	// summary: Returns the repository
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: reponame
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	iri := ctx.Repo.Repository.GetIRI()
	repo := forgefed.RepositoryNew(ap.IRI(iri))

	repo.Name = ap.NaturalLanguageValuesNew()
	err := repo.Name.Set("en", ap.Content(ctx.Repo.Repository.Name))
	if err != nil {
		ctx.ServerError("Set Name", err)
		return
	}

	repo.AttributedTo = ap.IRI(setting.AppURL + "api/v1/activitypub/user/" + ctx.ContextUser.Name)

	repo.Summary = ap.NaturalLanguageValuesNew()
	err = repo.Summary.Set("en", ap.Content(ctx.Repo.Repository.Description))
	if err != nil {
		ctx.ServerError("Set Description", err)
		return
	}

	repo.Inbox = ap.IRI(iri + "/inbox")
	repo.Outbox = ap.IRI(iri + "/outbox")
	repo.Followers = ap.IRI(iri + "/followers")
	repo.Team = ap.IRI(iri + "/team")

	response(ctx, repo)
}

// RepoInbox function handles the incoming data for a repo inbox
func RepoInbox(ctx *context.APIContext) {
	// swagger:operation POST /activitypub/repo/{username}/{reponame}/inbox activitypub activitypubRepoInbox
	// ---
	// summary: Send to the inbox
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: reponame
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.ServerError("Error reading request body", err)
		return
	}

	ap.ItemTyperFunc = forgefed.GetItemByType
	ap.JSONItemUnmarshal = forgefed.JSONUnmarshalerFn
	ap.NotEmptyChecker = forgefed.NotEmpty
	var activity ap.Activity
	err = activity.UnmarshalJSON(body)
	if err != nil {
		ctx.ServerError("UnmarshalJSON", err)
		return
	}

	// Make sure keyID matches the user doing the activity
	_, keyID, _ := getKeyID(ctx.Req)
	if activity.Actor != nil && !strings.HasPrefix(keyID, activity.Actor.GetLink().String()) {
		ctx.ServerError("Actor does not match HTTP signature keyID", nil)
		return
	}
	if activity.AttributedTo != nil && !strings.HasPrefix(keyID, activity.AttributedTo.GetLink().String()) {
		ctx.ServerError("AttributedTo does not match HTTP signature keyID", nil)
		return
	}

	if activity.Object == nil {
		ctx.ServerError("Activity does not contain object", err)
		return
	}

	// Process activity
	switch activity.Type {
	case ap.CreateType:
		switch activity.Object.GetType() {
		case forgefed.RepositoryType:
			// Fork created by remote instance
			err = forgefed.OnRepository(activity.Object, func(r *forgefed.Repository) error {
				return createRepository(ctx, r)
			})
		case forgefed.TicketType:
			// New issue or pull request
			err = forgefed.OnTicket(activity.Object, func(t *forgefed.Ticket) error {
				return createTicket(ctx, t)
			})
		case ap.NoteType:
			// New comment
			err = ap.On(activity.Object, func(n *ap.Note) error {
				return createComment(ctx, n)
			})
		default:
			log.Info("Incoming unsupported ActivityStreams object type: %s", activity.Object.GetType())
			ctx.PlainText(http.StatusNotImplemented, "ActivityStreams object type not supported")
			return
		}
	case ap.LikeType:
		err = star(ctx, activity)
	default:
		ctx.PlainText(http.StatusNotImplemented, "ActivityStreams type not supported")
		return
	}
	if err != nil {
		ctx.ServerError("Error when processing", err)
	}

	ctx.Status(http.StatusNoContent)
}

// RepoOutbox function returns the repo's Outbox OrderedCollection
func RepoOutbox(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/repo/{username}/{reponame}/outbox activitypub activitypubRepoOutbox
	// ---
	// summary: Returns the outbox
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: reponame
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	// TODO
	ctx.Status(http.StatusNotImplemented)
}

// RepoFollowers function returns the repo's Followers OrderedCollection
func RepoFollowers(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/repo/{username}/{reponame}/followers activitypub activitypubRepoFollowers
	// ---
	// summary: Returns the followers collection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: reponame
	//   in: path
	//   description: name of the repository
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	// TODO
	ctx.Status(http.StatusNotImplemented)
}

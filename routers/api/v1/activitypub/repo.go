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

	link := setting.AppURL + "api/v1/activitypub/repo/" + ctx.ContextUser.Name + "/" + ctx.Repo.Repository.Name
	repo := forgefed.RepositoryNew(ap.IRI(link))

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

	repo.Inbox = ap.IRI(link + "/inbox")
	repo.Outbox = ap.IRI(link + "/outbox")
	repo.Followers = ap.IRI(link + "/followers")
	repo.Team = ap.IRI(link + "/team")

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
		err = ap.OnObject(activity.Object, func(o *ap.Object) error {
			switch o.Type {
			case forgefed.RepositoryType:
				// Fork created by remote instance
				return fork(ctx, activity)
			case forgefed.TicketType:
				// New issue or pull request
				return forgefed.OnTicket(o, func(t *forgefed.Ticket) error {
					if t.Origin != nil {
						// New pull request
						return createPullRequest(ctx, t)
					}
					// New issue
					return createIssue(ctx, t)
				})
			case ap.NoteType:
				// New comment
				return createComment(ctx, o)
			}
			return nil
		})
	case ap.LikeType:
		err = star(ctx, activity)
	default:
		log.Info("Incoming unsupported ActivityStreams type: %s", activity.Type)
		ctx.PlainText(http.StatusNotImplemented, "ActivityStreams type not supported")
		return
	}
	if err != nil {
		ctx.ServerError("Error when processing: %s", err)
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

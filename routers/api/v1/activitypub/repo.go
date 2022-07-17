// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"io"
	"net/http"

	"code.gitea.io/gitea/models/forgefed"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
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

	var activity map[string]interface{}
	err = json.Unmarshal(body, activity)
	if err != nil {
		ctx.ServerError("Unmarshal", err)
		return
	}

	switch activity["type"].(ap.ActivityVocabularyType) {
	case ap.CreateType:
		// Create activity, extract the object
		object, ok := activity["object"].(map[string]interface{})
		if ok {
			ctx.ServerError("Activity does not contain object", err)
			return
		}
		objectBinary, err := json.Marshal(object)
		if err != nil {
			ctx.ServerError("Marshal", err)
			return
		}

		switch object["type"].(ap.ActivityVocabularyType) {
		case forgefed.RepositoryType:
			// Fork created by remote instance
			var repository forgefed.Repository
			repository.UnmarshalJSON(objectBinary)
			activitypub.ForkFromCreate(ctx, repository)
		case forgefed.TicketType:
			// New issue or pull request
			var ticket forgefed.Ticket
			ticket.UnmarshalJSON(objectBinary)
			if ticket.Origin != nil {
				// New pull request
				activitypub.PullRequest(ctx, ticket)
			} else {
				// New issue
				activitypub.Issue(ctx, ticket)
			}
		case ap.NoteType:
			// New comment
			var note ap.Note
			note.UnmarshalJSON(objectBinary)
			activitypub.Comment(ctx, note)
		}
	default:
		log.Info("Incoming unsupported ActivityStreams type: %s", activity["type"])
		ctx.PlainText(http.StatusNotImplemented, "ActivityStreams type not supported")
		return
	}

	ctx.Status(http.StatusNoContent)
}

// RepoOutbox function returns the repo's Outbox OrderedCollection
func RepoOutbox(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/repo/{username}/{reponame}/outbox activitypub activitypubPersonOutbox
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

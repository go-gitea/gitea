// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/forgefed"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/utils"

	ap "github.com/go-ap/activitypub"
)

// Repo function
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

	link := strings.TrimSuffix(setting.AppURL, "/") + "/api/v1/activitypub/repo/" + ctx.ContextUser.Name + "/" + ctx.Repo.Repository.Name
	repo := forgefed.RepositoryNew(ap.IRI(link))

	repo.Name = ap.NaturalLanguageValuesNew()
	err := repo.Name.Set("en", ap.Content(ctx.Repo.Repository.Name))
	if err != nil {
		ctx.ServerError("Set Name", err)
		return
	}

	repo.AttributedTo = ap.IRI(strings.TrimSuffix(setting.AppURL, "/") + "/api/v1/activitypub/user/" + ctx.ContextUser.Name)

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

// RepoInbox function
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
	}
	var activity ap.Activity
	activity.UnmarshalJSON(body) // This function doesn't support ForgeFed types!!!
	log.Warn("Debug", activity)
	switch activity.Type {
	case ap.NoteType:
		// activitypub.Comment(ctx, activity)
	case ap.CreateType:
		// if activity.Object.GetType() == forgefed.RepositoryType {
		// Fork created by remote instance
		activitypub.ForkFromCreate(ctx, activity)
		//}
	case ap.MoveType:
		// This should actually be forgefed.TicketType but that the UnmarshalJSON function above doesn't support ForgeFed!
		activitypub.PullRequest(ctx, activity)
	default:
		log.Warn("ActivityStreams type not supported", activity)
	}

	ctx.Status(http.StatusNoContent)
}

// RepoOutbox function
func RepoOutbox(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/repo/{username}/outbox activitypub activitypubPersonOutbox
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

	link := strings.TrimSuffix(setting.AppURL, "/") + "/api/v1/activitypub/repo/" + ctx.ContextUser.Name + "/" + ctx.Repo.Repository.Name

	feed, err := models.GetFeeds(ctx, models.GetFeedsOptions{
		RequestedUser:   ctx.ContextUser,
		Actor:           ctx.ContextUser,
		IncludePrivate:  false,
		OnlyPerformedBy: true,
		IncludeDeleted:  false,
		Date:            ctx.FormString("date"),
	})
	if err != nil {
		ctx.ServerError("Couldn't fetch outbox", err)
	}

	outbox := ap.OrderedCollectionNew(ap.IRI(link + "/outbox"))
	for _, action := range feed {
		/*if action.OpType == ExampleType {
			activity := ap.ExampleNew()
			outbox.OrderedItems.Append(activity)
		}*/
		log.Debug(action.Content)
	}
	outbox.TotalItems = uint(len(outbox.OrderedItems))

	response(ctx, outbox)
}

// RepoFollowers function
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

	link := strings.TrimSuffix(setting.AppURL, "/") + "/api/v1/activitypub/repo/" + ctx.ContextUser.Name + "/" + ctx.Repo.Repository.Name

	users, _, err := user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, utils.GetListOptions(ctx))
	if err != nil {
		ctx.ServerError("GetUserFollowers", err)
		return
	}

	followers := ap.OrderedCollectionNew(ap.IRI(link + "/followers"))
	followers.TotalItems = uint(len(users))

	for _, user := range users {
		person := ap.PersonNew(ap.IRI(user.Website))
		followers.OrderedItems.Append(person)
	}

	response(ctx, followers)
}

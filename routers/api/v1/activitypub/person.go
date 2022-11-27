// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
)

// Person function returns the Person actor for a user
func Person(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username} activitypub activitypubPerson
	// ---
	// summary: Returns the Person actor for a user
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	iri := ctx.ContextUser.GetIRI()
	person := ap.PersonNew(ap.IRI(iri))

	person.Name = ap.NaturalLanguageValuesNew()
	err := person.Name.Set("en", ap.Content(ctx.ContextUser.FullName))
	if err != nil {
		ctx.ServerError("Set Name", err)
		return
	}

	person.PreferredUsername = ap.NaturalLanguageValuesNew()
	err = person.PreferredUsername.Set("en", ap.Content(ctx.ContextUser.Name))
	if err != nil {
		ctx.ServerError("Set PreferredUsername", err)
		return
	}

	person.URL = ap.IRI(ctx.ContextUser.HTMLURL())
	person.Location = ap.IRI(ctx.ContextUser.GetEmail())

	person.Icon = ap.Image{
		Type:      ap.ImageType,
		MediaType: "image/png",
		URL:       ap.IRI(ctx.ContextUser.AvatarFullLinkWithSize(2048)),
	}

	person.Inbox = ap.IRI(iri + "/inbox")
	person.Outbox = ap.IRI(iri + "/outbox")
	person.Following = ap.IRI(iri + "/following")
	person.Followers = ap.IRI(iri + "/followers")
	person.Liked = ap.IRI(iri + "/liked")

	person.PublicKey.ID = ap.IRI(iri + "#main-key")
	person.PublicKey.Owner = ap.IRI(iri)
	publicKeyPem, err := activitypub.GetPublicKey(ctx.ContextUser)
	if err != nil {
		ctx.ServerError("GetPublicKey", err)
		return
	}
	person.PublicKey.PublicKeyPem = publicKeyPem

	response(ctx, person)
}

// PersonInbox function handles the incoming data for a user inbox
func PersonInbox(ctx *context.APIContext) {
	// swagger:operation POST /activitypub/user/{username}/inbox activitypub activitypubPersonInbox
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
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	body, err := io.ReadAll(io.LimitReader(ctx.Req.Body, setting.Federation.MaxSize))
	if err != nil {
		ctx.ServerError("Error reading request body", err)
		return
	}

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
	// TODO: Check activity.Object actor and attributedTo

	// Process activity
	switch activity.Type {
	case ap.FollowType:
		err = follow(ctx, activity)
	case ap.UndoType:
		err = unfollow(ctx, activity)
	default:
		log.Info("Incoming unsupported ActivityStreams type: %s", activity.GetType())
		ctx.PlainText(http.StatusNotImplemented, "ActivityStreams type not supported")
		return
	}
	if err != nil {
		ctx.ServerError("Could not process activity", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// PersonOutbox function returns the user's Outbox OrderedCollection
func PersonOutbox(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/outbox activitypub activitypubPersonOutbox
	// ---
	// summary: Returns the Outbox OrderedCollection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	iri := ctx.ContextUser.GetIRI()

	orderedCollection := ap.OrderedCollectionNew(ap.IRI(iri + "/outbox"))
	orderedCollection.First = ap.IRI(iri + "/outbox?page=1")

	outbox := ap.OrderedCollectionPageNew(orderedCollection)
	outbox.First = ap.IRI(iri + "/outbox?page=1")

	feed, err := activities.GetFeeds(ctx, activities.GetFeedsOptions{
		RequestedUser:       ctx.ContextUser,
		RequestedActionType: activities.ActionCreateRepo,
		Actor:               ctx.Doer,
		IncludePrivate:      false,
		IncludeDeleted:      false,
		ListOptions:         utils.GetListOptions(ctx),
	})

	// Only specify next if this amount of feed corresponds to the calculated limit.
	if len(feed) == convert.ToCorrectPageSize(ctx.FormInt("limit")) {
		outbox.Next = ap.IRI(fmt.Sprintf("%s/outbox?page=%d", iri, ctx.FormInt("page")+1))
	}

	// Only specify previous page when there is one.
	if ctx.FormInt("page") > 1 {
		outbox.Prev = ap.IRI(fmt.Sprintf("%s/outbox?page=%d", iri, ctx.FormInt("page")-1))
	}

	if err != nil {
		ctx.ServerError("Couldn't fetch feed", err)
		return
	}

	for _, action := range feed {
		// Created a repo
		object := ap.Note{Type: ap.NoteType, Content: ap.NaturalLanguageValuesNew()}
		_ = object.Content.Set("en", ap.Content(action.GetRepoName()))
		create := ap.Create{Type: ap.CreateType, Object: object}
		err := outbox.OrderedItems.Append(create)
		if err != nil {
			ctx.ServerError("OrderedItems.Append", err)
			return
		}
	}

	// TODO: Remove this code and implement an ActionStarRepo type, so `GetFeeds`
	// can handle this with correct pagination and ordering.
	stars, err := repo_model.GetStarredRepos(ctx.ContextUser.ID, false, db.ListOptions{Page: 1, PageSize: 1000000})
	if err != nil {
		ctx.ServerError("Couldn't fetch stars", err)
		return
	}

	for _, star := range stars {
		object := ap.Note{Type: ap.NoteType, Content: ap.NaturalLanguageValuesNew()}
		_ = object.Content.Set("en", ap.Content("Starred "+star.Name))
		create := ap.Create{Type: ap.CreateType, Object: object}
		err := outbox.OrderedItems.Append(create)
		if err != nil {
			ctx.ServerError("OrderedItems.Append", err)
			return
		}
	}

	outbox.TotalItems = uint(len(outbox.OrderedItems))

	response(ctx, outbox)
}

// PersonFollowing function returns the user's Following Collection
func PersonFollowing(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/following activitypub activitypubPersonFollowing
	// ---
	// summary: Returns the Following Collection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	iri := ctx.ContextUser.GetIRI()

	users, _, err := user_model.GetUserFollowing(ctx, ctx.ContextUser, ctx.Doer, utils.GetListOptions(ctx))
	if err != nil {
		ctx.ServerError("GetUserFollowing", err)
		return
	}

	following := ap.OrderedCollectionNew(ap.IRI(iri + "/following"))
	following.TotalItems = uint(len(users))

	for _, user := range users {
		// TODO: handle non-Federated users
		person := ap.PersonNew(ap.IRI(user.Website))
		err := following.OrderedItems.Append(person)
		if err != nil {
			ctx.ServerError("OrderedItems.Append", err)
			return
		}
	}

	response(ctx, following)
}

// PersonFollowers function returns the user's Followers Collection
func PersonFollowers(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/followers activitypub activitypubPersonFollowers
	// ---
	// summary: Returns the Followers Collection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	iri := ctx.ContextUser.GetIRI()

	users, _, err := user_model.GetUserFollowers(ctx, ctx.ContextUser, ctx.Doer, utils.GetListOptions(ctx))
	if err != nil {
		ctx.ServerError("GetUserFollowers", err)
		return
	}

	followers := ap.OrderedCollectionNew(ap.IRI(iri + "/followers"))
	followers.TotalItems = uint(len(users))

	for _, user := range users {
		// TODO: handle non-Federated users
		person := ap.PersonNew(ap.IRI(user.Website))
		err := followers.OrderedItems.Append(person)
		if err != nil {
			ctx.ServerError("OrderedItems.Append", err)
			return
		}
	}

	response(ctx, followers)
}

// PersonLiked function returns the user's Liked Collection
func PersonLiked(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/followers activitypub activitypubPersonLiked
	// ---
	// summary: Returns the Liked Collection
	// produces:
	// - application/activity+json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	iri := ctx.ContextUser.GetIRI()

	repos, count, err := repo_model.SearchRepository(&repo_model.SearchRepoOptions{
		Actor:       ctx.Doer,
		Private:     ctx.IsSigned,
		StarredByID: ctx.ContextUser.ID,
	})
	if err != nil {
		ctx.ServerError("GetUserStarred", err)
		return
	}

	liked := ap.OrderedCollectionNew(ap.IRI(iri + "/liked"))
	liked.TotalItems = uint(count)

	for _, repo := range repos {
		// TODO: Handle remote starred repos
		repo := forgefed.RepositoryNew(ap.IRI(setting.AppURL + "api/v1/activitypub/repo/" + repo.OwnerName + "/" + repo.Name))
		err := liked.OrderedItems.Append(repo)
		if err != nil {
			ctx.ServerError("OrderedItems.Append", err)
			return
		}
	}

	response(ctx, liked)
}

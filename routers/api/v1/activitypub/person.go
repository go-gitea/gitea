// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/utils"

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

	link := strings.TrimSuffix(setting.AppURL, "/") + "/api/v1/activitypub/user/" + ctx.ContextUser.Name
	person := ap.PersonNew(ap.IRI(link))

	person.Name = ap.NaturalLanguageValuesNew()
	err := person.Name.Set("en", ap.Content(ctx.ContextUser.FullName))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Set Name", err)
		return
	}

	person.PreferredUsername = ap.NaturalLanguageValuesNew()
	err = person.PreferredUsername.Set("en", ap.Content(ctx.ContextUser.Name))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Set PreferredUsername", err)
		return
	}

	person.URL = ap.IRI(ctx.ContextUser.HTMLURL())

	person.Icon = ap.Image{
		Type:      ap.ImageType,
		MediaType: "image/png",
		URL:       ap.IRI(ctx.ContextUser.AvatarLink()),
	}

	person.Inbox = ap.IRI(link + "/inbox")
	person.Outbox = ap.IRI(link + "/outbox")

	person.Following = ap.IRI(link + "/following")
	person.Followers = ap.IRI(link + "/followers")

	person.PublicKey.ID = ap.IRI(link + "#main-key")
	person.PublicKey.Owner = ap.IRI(link)

	publicKeyPem, err := activitypub.GetPublicKey(ctx.ContextUser)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPublicKey", err)
		return
	}
	person.PublicKey.PublicKeyPem = publicKeyPem

	binary, err := person.MarshalJSON()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Serialize", err)
		return
	}
	response(ctx, binary)
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

	body, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Error reading request body", err)
	}

	var activity ap.Activity
	activity.UnmarshalJSON(body)
	if activity.Type == ap.FollowType {
		activitypub.Follow(ctx, activity)
	} else {
		log.Warn("ActivityStreams type not supported", activity)
	}

	ctx.Status(http.StatusNoContent)
}

// PersonOutbox function
func PersonOutbox(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/outbox activitypub activitypubPersonOutbox
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	link := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ctx.Req.URL.EscapedPath(), "/")

	feed, err := models.GetFeeds(ctx, models.GetFeedsOptions{
		RequestedUser:   ctx.ContextUser,
		Actor:           ctx.ContextUser,
		IncludePrivate:  false,
		OnlyPerformedBy: true,
		IncludeDeleted:  false,
		Date:            ctx.FormString("date"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Couldn't fetch outbox", err)
	}

	outbox := ap.OrderedCollectionNew(ap.IRI(link))
	for _, action := range feed {
		/*if action.OpType == ExampleType {
			activity := ap.ExampleNew()
			outbox.OrderedItems.Append(activity)
		}*/
		log.Debug(action.Content)
	}
	outbox.TotalItems = uint(len(outbox.OrderedItems))

	binary, err := outbox.MarshalJSON()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Serialize", err)
	}
	response(ctx, binary)
}

// PersonFollowing function
func PersonFollowing(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/following activitypub activitypubPersonFollowing
	// ---
	// summary: Returns the following collection
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

	link := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ctx.Req.URL.EscapedPath(), "/")

	users, err := user_model.GetUserFollowing(ctx.ContextUser, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserFollowing", err)
		return
	}

	following := ap.OrderedCollectionNew(ap.IRI(link))
	following.TotalItems = uint(len(users))

	for _, user := range users {
		// TODO: handle non-Federated users
		person := ap.PersonNew(ap.IRI(user.Website))
		following.OrderedItems.Append(person)
	}

	binary, err := following.MarshalJSON()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Serialize", err)
	}
	response(ctx, binary)
}

// PersonFollowers function
func PersonFollowers(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username}/followers activitypub activitypubPersonFollowers
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	link := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ctx.Req.URL.EscapedPath(), "/")

	users, err := user_model.GetUserFollowers(ctx.ContextUser, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserFollowers", err)
		return
	}

	followers := ap.OrderedCollectionNew(ap.IRI(link))
	followers.TotalItems = uint(len(users))

	for _, user := range users {
		person := ap.PersonNew(ap.IRI(user.Website))
		followers.OrderedItems.Append(person)
	}

	binary, err := followers.MarshalJSON()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Serialize", err)
	}
	response(ctx, binary)
}

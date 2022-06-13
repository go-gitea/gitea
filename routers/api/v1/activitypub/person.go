// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/user"

	ap "github.com/go-ap/activitypub"
)

// Person function
func Person(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username} activitypub activitypubPerson
	// ---
	// summary: Returns the person
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

	user := user.GetUserByParamsName(ctx, "username")
	if user == nil {
		return
	}
	username := ctx.Params("username")

	link := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ctx.Req.URL.EscapedPath(), "/")
	person := ap.PersonNew(ap.IRI(link))

	person.Name = ap.NaturalLanguageValuesNew()
	err := person.Name.Set("en", ap.Content(user.FullName))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Set Name", err)
		return
	}

	person.PreferredUsername = ap.NaturalLanguageValuesNew()
	err = person.PreferredUsername.Set("en", ap.Content(username))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Set PreferredUsername", err)
		return
	}
	
	person.URL = ap.IRI(setting.AppURL + username)

	person.Icon = ap.Image{
		Type: ap.ImageType,
		MediaType: "image/png",
		URL: ap.IRI(user.AvatarLink()),
	}

	person.Inbox = nil
	person.Inbox, _ = ap.Inbox.AddTo(person)
	person.Outbox = nil
	person.Outbox, _ = ap.Outbox.AddTo(person)

	person.PublicKey.ID = ap.IRI(link + "#main-key")
	person.PublicKey.Owner = ap.IRI(link)

	publicKeyPem, err := activitypub.GetPublicKey(user)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPublicKey", err)
		return
	}
	person.PublicKey.PublicKeyPem = publicKeyPem

	binary, err := person.MarshalJSON()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Serialize", err)
	}

	var jsonmap map[string]interface{}
	err = json.Unmarshal(binary, &jsonmap)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Unmarshall", err)
	}

	jsonmap["@context"] = []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"} 

	ctx.Resp.Header().Add("Content-Type", "application/activity+json")
	ctx.Resp.WriteHeader(http.StatusOK)
	binary, _ = json.Marshal(jsonmap)
	ctx.Resp.Write(binary)
}

// PersonInbox function
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
	/* if activity.Type == ap.ExampleType {

	}*/
	log.Warn("ActivityStreams type not supported", activity)

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

	user := user.GetUserByParamsName(ctx, "username")
	if user == nil {
		return
	}
	link := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ctx.Req.URL.EscapedPath(), "/")

	feed, err := models.GetFeeds(ctx, models.GetFeedsOptions{
		RequestedUser:   user,
		Actor:           user,
		IncludePrivate:  false,
		OnlyPerformedBy: true,
		IncludeDeleted:  false,
		Date:            ctx.FormString("date"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Couldn't fetch outbox", err)
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

	binary, err := outbox.MarshalJSON()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Serialize", err)
	}

	var jsonmap map[string]interface{}
	err = json.Unmarshal(binary, &jsonmap)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Unmarshall", err)
	}

	jsonmap["@context"] = "https://www.w3.org/ns/activitystreams"

	ctx.JSON(http.StatusOK, jsonmap)
}

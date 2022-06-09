// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
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
	// - application/json
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

	name := ap.NaturalLanguageValuesNew()
	err := name.Set("en", ap.Content(username))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Set name", err)
		return
	}
	person.Name = name

	person.Inbox = ap.Item(ap.IRI(link + "/inbox"))
	person.Outbox = ap.Item(ap.IRI(link + "/outbox"))

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
	err = json.Unmarshal(binary, jsonmap)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Unmarshall", err)
	}

	ctx.JSON(http.StatusOK, jsonmap)
}

// PersonInbox function
func PersonInbox(ctx *context.APIContext) {
	// swagger:operation POST /activitypub/user/{username}/inbox activitypub activitypubPersonInbox
	// ---
	// summary: Send to the inbox
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// responses:
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	ctx.Status(http.StatusNoContent)
}

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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

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

	var jsonmap map[string]interface{}
	err = json.Unmarshal(binary, &jsonmap)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Unmarshal", err)
		return
	}

	jsonmap["@context"] = []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"}

	ctx.Resp.Header().Add("Content-Type", activitypub.ActivityStreamsContentType)
	ctx.Resp.WriteHeader(http.StatusOK)
	binary, err = json.Marshal(jsonmap)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Marshal", err)
		return
	}
	if _, err = ctx.Resp.Write(binary); err != nil {
		log.Error("write to resp err: %v", err)
	}
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

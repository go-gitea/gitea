// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activitypub

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
)

// Person function returns the Person actor for a user
func Person(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user-id/{user-id} activitypub activitypubPerson
	// ---
	// summary: Returns the Person actor for a user
	// produces:
	// - application/json
	// parameters:
	// - name: user-id
	//   in: path
	//   description: user ID of the user
	//   type: integer
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ActivityPub"

	// TODO: the setting.AppURL during the test doesn't follow the definition: "It always has a '/' suffix"
	link := fmt.Sprintf("%s/api/v1/activitypub/user-id/%d", strings.TrimSuffix(setting.AppURL, "/"), ctx.ContextUser.ID)
	person := ap.PersonNew(ap.IRI(link))

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

	person.Icon = ap.Image{
		Type:      ap.ImageType,
		MediaType: "image/png",
		URL:       ap.IRI(ctx.ContextUser.AvatarLink(ctx)),
	}

	person.Inbox = ap.IRI(link + "/inbox")
	person.Outbox = ap.IRI(link + "/outbox")

	person.PublicKey.ID = ap.IRI(link + "#main-key")
	person.PublicKey.Owner = ap.IRI(link)

	publicKeyPem, err := activitypub.GetPublicKey(ctx, ctx.ContextUser)
	if err != nil {
		ctx.ServerError("GetPublicKey", err)
		return
	}
	person.PublicKey.PublicKeyPem = publicKeyPem

	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(person)
	if err != nil {
		ctx.ServerError("MarshalJSON", err)
		return
	}
	ctx.Resp.Header().Add("Content-Type", activitypub.ActivityStreamsContentType)
	ctx.Resp.WriteHeader(http.StatusOK)
	if _, err = ctx.Resp.Write(binary); err != nil {
		log.Error("write to resp err: %v", err)
	}
}

// PersonInbox function handles the incoming data for a user inbox
func PersonInbox(ctx *context.APIContext) {
	// swagger:operation POST /activitypub/user-id/{user-id}/inbox activitypub activitypubPersonInbox
	// ---
	// summary: Send to the inbox
	// produces:
	// - application/json
	// parameters:
	// - name: user-id
	//   in: path
	//   description: user ID of the user
	//   type: integer
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	ctx.Status(http.StatusNoContent)
}

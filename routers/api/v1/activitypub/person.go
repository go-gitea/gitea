// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/user"

	"github.com/go-fed/activity/streams"
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

	person := streams.NewActivityStreamsPerson()

	id := streams.NewJSONLDIdProperty()
	link := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ctx.Req.URL.EscapedPath(), "/")
	idIRI, _ := url.Parse(link)
	id.SetIRI(idIRI)
	person.SetJSONLDId(id)

	name := streams.NewActivityStreamsNameProperty()
	name.AppendXMLSchemaString(username)
	person.SetActivityStreamsName(name)

	ibox := streams.NewActivityStreamsInboxProperty()
	urlObject, _ := url.Parse(link + "/inbox")
	ibox.SetIRI(urlObject)
	person.SetActivityStreamsInbox(ibox)

	obox := streams.NewActivityStreamsOutboxProperty()
	urlObject, _ = url.Parse(link + "/outbox")
	obox.SetIRI(urlObject)
	person.SetActivityStreamsOutbox(obox)

	publicKeyProp := streams.NewW3IDSecurityV1PublicKeyProperty()

	publicKeyType := streams.NewW3IDSecurityV1PublicKey()

	pubKeyIDProp := streams.NewJSONLDIdProperty()
	pubKeyIRI, _ := url.Parse(link + "#main-key")
	pubKeyIDProp.SetIRI(pubKeyIRI)
	publicKeyType.SetJSONLDId(pubKeyIDProp)

	ownerProp := streams.NewW3IDSecurityV1OwnerProperty()
	ownerProp.SetIRI(idIRI)
	publicKeyType.SetW3IDSecurityV1Owner(ownerProp)

	publicKeyPemProp := streams.NewW3IDSecurityV1PublicKeyPemProperty()
	if publicKeyPem, err := activitypub.GetPublicKey(user); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPublicKey", err)
		return
	} else {
		publicKeyPemProp.Set(publicKeyPem)
	}
	publicKeyType.SetW3IDSecurityV1PublicKeyPem(publicKeyPemProp)

	publicKeyProp.AppendW3IDSecurityV1PublicKey(publicKeyType)
	person.SetW3IDSecurityV1PublicKey(publicKeyProp)

	jsonmap, err := streams.Serialize(person)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Serialize", err)
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

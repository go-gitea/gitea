// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/user"
	"github.com/go-fed/activity/streams"
)

// hack waiting on https://github.com/go-gitea/gitea/pull/16834
func GetPublicKey(user *models.User) (string, error) {
	if settings, err := models.GetUserSetting(user.ID, []string{"activitypub_pubPem"}); err != nil {
		return "", err
	} else if len(settings) == 0 {
		if priv, pub, err := activitypub.GenerateKeyPair(); err != nil {
			return "", err
		} else {
			privPem := &models.UserSetting{UserID: user.ID, Name: "activitypub_privPem", Value: priv}
			if err := models.SetUserSetting(privPem); err != nil {
				return "", err
			}
			pubPem := &models.UserSetting{UserID: user.ID, Name: "activitypub_pubPem", Value: pub}
			if err := models.SetUserSetting(pubPem); err != nil {
				return "", err
			}
			return pubPem.Value, nil
		}
	} else {
		return settings[0].Value, nil
	}
}

// NodeInfo returns the NodeInfo for the Gitea instance to allow for federation
func Person(ctx *context.APIContext) {
	// swagger:operation GET /activitypub/user/{username} information
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
	url_object, _ := url.Parse(link + "/inbox")
	ibox.SetIRI(url_object)
	person.SetActivityStreamsInbox(ibox)

	obox := streams.NewActivityStreamsOutboxProperty()
	url_object, _ = url.Parse(link + "/outbox")
	obox.SetIRI(url_object)
	person.SetActivityStreamsOutbox(obox)

	publicKeyProp := streams.NewW3IDSecurityV1PublicKeyProperty()

	publicKeyType := streams.NewW3IDSecurityV1PublicKey()

	pubKeyIdProp := streams.NewJSONLDIdProperty()
	pubKeyIRI, _ := url.Parse(link + "/#main-key")
	pubKeyIdProp.SetIRI(pubKeyIRI)
	publicKeyType.SetJSONLDId(pubKeyIdProp)

	ownerProp := streams.NewW3IDSecurityV1OwnerProperty()
	ownerProp.SetIRI(idIRI)
	publicKeyType.SetW3IDSecurityV1Owner(ownerProp)

	publicKeyPemProp := streams.NewW3IDSecurityV1PublicKeyPemProperty()
	if publicKeyPem, err := GetPublicKey(user); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetPublicKey", err)
	} else {
		publicKeyPemProp.Set(publicKeyPem)
	}
	publicKeyType.SetW3IDSecurityV1PublicKeyPem(publicKeyPemProp)

	publicKeyProp.AppendW3IDSecurityV1PublicKey(publicKeyType)
	person.SetW3IDSecurityV1PublicKey(publicKeyProp)

	var jsonmap map[string]interface{}
	jsonmap, _ = streams.Serialize(person)
	ctx.JSON(http.StatusOK, jsonmap)
}

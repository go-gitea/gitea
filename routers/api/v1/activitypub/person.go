// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/v1/user"
	"github.com/go-fed/activity/streams"
)

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

	user.GetUserByParamsName(ctx, "username")
	username := ctx.Params("username")

	person := streams.NewActivityStreamsPerson()

	id := streams.NewJSONLDIdProperty()
	link := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ctx.Req.URL.EscapedPath(), "/")
	url_object, _ := url.Parse(link)
	id.SetIRI(url_object)
	person.SetJSONLDId(id)

	name := streams.NewActivityStreamsNameProperty()
	name.AppendXMLSchemaString(username)
	person.SetActivityStreamsName(name)

	ibox := streams.NewActivityStreamsInboxProperty()
	url_object, _ = url.Parse(link + "/inbox")
	ibox.SetIRI(url_object)
	person.SetActivityStreamsInbox(ibox)

	obox := streams.NewActivityStreamsOutboxProperty()
	url_object, _ = url.Parse(link + "/outbox")
	obox.SetIRI(url_object)
	person.SetActivityStreamsOutbox(obox)

	var jsonmap map[string]interface{}
	jsonmap, _ = streams.Serialize(person)
	ctx.JSON(http.StatusOK, jsonmap)
}

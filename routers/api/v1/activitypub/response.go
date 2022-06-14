// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"

	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
)

func response(ctx *context.APIContext, binary []byte) {
	var jsonmap map[string]interface{}
	err := json.Unmarshal(binary, &jsonmap)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Unmarshall", err)
	}

	jsonmap["@context"] = []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/v1"} 

	ctx.Resp.Header().Add("Content-Type", activitypub.ActivityStreamsContentType)
	ctx.Resp.WriteHeader(http.StatusOK)
	binary, _ = json.Marshal(jsonmap)
	ctx.Resp.Write(binary)
}
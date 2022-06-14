// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"

	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
)

func response(ctx *context.APIContext, binary []byte) {
	var jsonmap map[string]interface{}
	err := json.Unmarshal(binary, &jsonmap)
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

// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/activitypub"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
)

// Respond with an ActivityStreams object
func response(ctx *context.APIContext, v interface{}) {
	binary, err := jsonld.WithContext(
		jsonld.IRI(ap.ActivityBaseURI),
		jsonld.IRI(ap.SecurityContextURI),
		jsonld.IRI(forgefed.ForgeFedNamespaceURI),
	).Marshal(v)
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	ctx.Resp.Header().Add("Content-Type", activitypub.ActivityStreamsContentType)
	ctx.Resp.WriteHeader(http.StatusOK)
	if _, err = ctx.Resp.Write(binary); err != nil {
		log.Error("write to resp err: %v", err)
	}
}

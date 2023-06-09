// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/services/auth/source/saml"
)

// SignInSAML
func SignInSAML(ctx *context.Context) {
	provider := ctx.Params(":provider")

	loginSource, err := auth.GetActiveSAMLLoginSourceByName(provider)
	if err != nil || loginSource == nil {
		ctx.NotFound("SAMLMetadata", err)
		return
	}

	if err = loginSource.Cfg.(*saml.Source).Callout(ctx.Req, ctx.Resp); err != nil {
		if strings.Contains(err.Error(), "no provider for ") {
			ctx.Error(http.StatusNotFound)
			return
		}
		ctx.ServerError("SignIn", err)
	}
}

// SignInSAMLCallback
func SignInSAMLCallback(ctx *context.Context) {
	// provider := ctx.Params(":provider")
	// TODO: complete SAML Callback
}

// SAMLMetadata
func SAMLMetadata(ctx *context.Context) {
	provider := ctx.Params(":provider")
	loginSource, err := auth.GetActiveSAMLLoginSourceByName(provider)
	if err != nil || loginSource == nil {
		ctx.NotFound("SAMLMetadata", err)
		return
	}
	if err = loginSource.Cfg.(*saml.Source).Metadata(ctx.Req, ctx.Resp); err != nil {
		ctx.ServerError("SAMLMetadata", err)
	}
}

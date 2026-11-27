// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	"gitea.dev/modules/auth/httpauth"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
)

func oidcConfiguration(resp http.ResponseWriter, req *http.Request) {
	issuer := actions_service.OIDCIssuer()
	context.NewBaseContext(resp, req).JSON(http.StatusOK, map[string]any{
		"issuer":                                issuer,
		"jwks_uri":                              issuer + "/jwks",
		"response_types_supported":              []string{"id_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{actions_service.OIDCSigningAlgorithm},
		"claims_supported":                      actions_service.OIDCClaimsSupported(),
		"scopes_supported":                      []string{"openid"},
	})
}

func oidcKeys(resp http.ResponseWriter, req *http.Request) {
	ctx := context.NewBaseContext(resp, req)
	key, err := actions_service.OIDCSigningKey()
	if err != nil {
		log.Error("Actions OIDC signing key: %v", err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}
	jwk, err := key.ToJWK()
	if err != nil {
		log.Error("Actions OIDC JWK: %v", err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}
	jwk["use"] = "sig"
	ctx.JSON(http.StatusOK, map[string]any{"keys": []map[string]string{jwk}})
}

func oidcToken(resp http.ResponseWriter, req *http.Request) {
	ctx := context.NewBaseContext(resp, req)
	token := ""
	if auth, ok := httpauth.ParseAuthorizationHeader(req.Header.Get("Authorization")); ok && auth.BearerToken != nil {
		token = auth.BearerToken.Token
	}
	idToken, err := actions_service.CreateOIDCToken(ctx, token, ctx.FormString("audience"))
	if errors.Is(err, util.ErrPermissionDenied) {
		ctx.Resp.Header().Set("WWW-Authenticate", "Bearer")
		ctx.HTTPError(http.StatusUnauthorized)
		return
	} else if err != nil {
		log.Error("Actions OIDC token: %v", err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}
	ctx.Resp.Header().Set("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, map[string]string{"value": idToken})
}

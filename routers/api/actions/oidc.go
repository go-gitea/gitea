// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/auth/httpauth"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	actions_service "gitea.dev/services/actions"
	"gitea.dev/services/context"
	"gitea.dev/services/oauth2_provider"
)

var errInvalidOIDCAuthorization = errors.New("invalid authorization token")

func registerOIDCRoutes(m *web.Router) {
	m.Group("/oidc", func() {
		m.Get("/.well-known/openid-configuration", oidcWellKnown)
		m.Get("/jwks", oidcKeys)
		m.Get("/token", oidcToken)
	})
}

func oidcWellKnown(resp http.ResponseWriter, req *http.Request) {
	ctx := context.NewBaseContext(resp, req)
	if !actions_service.OIDCEnabled() {
		ctx.HTTPError(http.StatusNotFound)
		return
	}
	issuer := actions_service.OIDCIssuer()
	signingKey := oauth2_provider.DefaultSigningKey

	ctx.JSON(http.StatusOK, map[string]any{
		"issuer":                                issuer,
		"jwks_uri":                              issuer + "/jwks",
		"response_types_supported":              []string{"id_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{signingKey.SigningMethod().Alg()},
		"scopes_supported":                      []string{"openid"},
		"claims_supported": []string{
			"aud", "exp", "iat", "iss", "jti", "nbf", "sub",
			"actor", "actor_id", "repository", "repository_id", "repository_owner", "repository_owner_id",
			"run_id", "run_number", "run_attempt", "workflow", "workflow_repository", "workflow_repository_id", "workflow_ref", "workflow_sha",
			"job_workflow_repository", "job_workflow_repository_id", "job_workflow_ref", "job_workflow_sha", "repository_visibility", "event_name",
			"ref", "ref_type", "sha", "job_id", "base_ref", "head_ref", "runner_environment",
		},
	})
}

func oidcKeys(resp http.ResponseWriter, req *http.Request) {
	ctx := context.NewBaseContext(resp, req)
	if !actions_service.OIDCEnabled() {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	jwk, err := oauth2_provider.DefaultSigningKey.ToJWK()
	if err != nil {
		log.Error("Error converting Actions OIDC signing key to JWK: %v", err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}
	jwk["use"] = "sig"
	ctx.Resp.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(ctx.Resp).Encode(map[string][]map[string]string{"keys": {jwk}}); err != nil {
		log.Error("Failed to encode Actions OIDC JWKS response: %v", err)
	}
}

func oidcToken(resp http.ResponseWriter, req *http.Request) {
	ctx := context.NewBaseContext(resp, req)
	if !actions_service.OIDCEnabled() {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	task, err := getTaskFromOIDCTokenRequest(ctx)
	if err != nil {
		if errors.Is(err, errInvalidOIDCAuthorization) {
			ctx.Resp.Header().Set("WWW-Authenticate", `Bearer realm="Gitea Actions OIDC"`)
			ctx.HTTPError(http.StatusUnauthorized)
		} else {
			log.Error("Error getting Actions OIDC task: %v", err)
			ctx.HTTPError(http.StatusInternalServerError)
		}
		return
	}

	token, err := actions_service.CreateOIDCToken(ctx, task.ID, req.URL.Query().Get("audience"))
	if err != nil {
		switch {
		case errors.Is(err, actions_service.ErrOIDCInvalidAudience):
			ctx.HTTPError(http.StatusBadRequest, err.Error())
		case errors.Is(err, actions_service.ErrOIDCPermissionDenied), errors.Is(err, actions_service.ErrOIDCTaskNotRunning):
			ctx.Resp.Header().Set("WWW-Authenticate", `Bearer realm="Gitea Actions OIDC"`)
			ctx.HTTPError(http.StatusUnauthorized)
		default:
			log.Error("Error generating Actions OIDC token: %v", err)
			ctx.HTTPError(http.StatusInternalServerError)
		}
		return
	}

	ctx.Resp.Header().Set("Cache-Control", "no-store")
	ctx.Resp.Header().Set("Pragma", "no-cache")
	ctx.JSON(http.StatusOK, map[string]string{"value": token})
}

func getTaskFromOIDCTokenRequest(ctx *context.Base) (*actions_model.ActionTask, error) {
	parsed, ok := httpauth.ParseAuthorizationHeader(ctx.Req.Header.Get("Authorization"))
	if !ok || parsed.BearerToken == nil {
		return nil, errInvalidOIDCAuthorization
	}
	taskID, err := actions_service.TokenToTaskID(parsed.BearerToken.Token)
	if err != nil || taskID == 0 {
		return nil, errInvalidOIDCAuthorization
	}
	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return nil, errInvalidOIDCAuthorization
		}
		return nil, err
	}
	if task.Status != actions_model.StatusRunning {
		return nil, errInvalidOIDCAuthorization
	}
	return task, nil
}

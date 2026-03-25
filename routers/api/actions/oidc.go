// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	actions_service "code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/oauth2_provider"
)

func registerOIDCRoutes(m *web.Router) {
	m.Group("/oidc", func() {
		m.Get("/.well-known/openid-configuration", oidcWellKnown)
		m.Get("/jwks", oidcKeys)
		m.Get("/token", oidcToken)
	})
}

func oidcWellKnown(resp http.ResponseWriter, req *http.Request) {
	ctx := context.NewBaseContext(resp, req)
	if !setting.OAuth2.Enabled {
		ctx.HTTPError(http.StatusNotFound)
		return
	}
	issuer := actions_service.OIDCIssuer()
	signingKey := oauth2_provider.DefaultSigningKey
	if signingKey == nil {
		ctx.HTTPError(http.StatusInternalServerError, "OIDC signing key is not initialized")
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"issuer":                                issuer,
		"jwks_uri":                              issuer + "/jwks",
		"token_endpoint":                        issuer + "/token",
		"response_types_supported":              []string{"id_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{signingKey.SigningMethod().Alg()},
		"claims_supported": []string{
			"aud",
			"exp",
			"iat",
			"iss",
			"jti",
			"sub",
			"nbf",
			"actor",
			"actor_id",
			"repository",
			"repository_id",
			"repository_owner",
			"repository_owner_id",
			"run_id",
			"run_number",
			"run_attempt",
			"workflow",
			"workflow_ref",
			"workflow_sha",
			"job_workflow_ref",
			"job_workflow_sha",
			"repository_visibility",
			"event_name",
			"ref",
			"ref_type",
			"sha",
			"job_id",
			"job_attempt",
			"base_ref",
			"head_ref",
			"runner_environment",
			"environment",
		},
	})
}

func oidcKeys(resp http.ResponseWriter, req *http.Request) {
	ctx := context.NewBaseContext(resp, req)
	if !setting.OAuth2.Enabled {
		ctx.HTTPError(http.StatusNotFound)
		return
	}
	signingKey := oauth2_provider.DefaultSigningKey
	if signingKey == nil {
		ctx.HTTPError(http.StatusInternalServerError, "OIDC signing key is not initialized")
		return
	}

	jwk, err := signingKey.ToJWK()
	if err != nil {
		log.Error("Error converting signing key to JWK: %v", err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}

	jwk["use"] = "sig"
	ctx.Resp.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(ctx.Resp).Encode(map[string][]map[string]string{"keys": {jwk}}); err != nil {
		log.Error("Failed to encode OIDC JWKS response: %v", err)
	}
}

func oidcToken(resp http.ResponseWriter, req *http.Request) {
	ctx := context.NewBaseContext(resp, req)
	if !setting.OAuth2.Enabled {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	task, err := getTaskFromOIDCTokenRequest(ctx)
	if err != nil {
		ctx.HTTPError(http.StatusUnauthorized, err.Error())
		return
	}
	if err := task.LoadAttributes(ctx); err != nil {
		log.Error("Error runner api getting task attributes: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error runner api getting task attributes")
		return
	}

	query := req.URL.Query()
	if runID := query.Get("run_id"); runID != "" {
		if runID != strconv.FormatInt(task.Job.RunID, 10) {
			ctx.HTTPError(http.StatusUnauthorized, "OIDC run_id mismatch")
			return
		}
	}
	if jobID := query.Get("job_id"); jobID != "" {
		if jobID != strconv.FormatInt(task.Job.ID, 10) {
			ctx.HTTPError(http.StatusUnauthorized, "OIDC job_id mismatch")
			return
		}
	}

	allowed, err := actions_service.TaskAllowsOIDCToken(ctx, task)
	if err != nil {
		log.Error("Error checking OIDC token permissions: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error checking OIDC permissions")
		return
	}
	if !allowed {
		ctx.HTTPError(http.StatusForbidden, "OIDC token permission not granted")
		return
	}

	audience := query.Get("audience")
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(actions_service.OIDCTokenExpiry())
	token, err := actions_service.CreateOIDCToken(ctx, task, audience)
	if err != nil {
		log.Error("Error generating OIDC token: %v", err)
		ctx.HTTPError(http.StatusInternalServerError, "Error generating OIDC token")
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"value":      token,
		"issued_at":  issuedAt.Format(time.RFC3339),
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func getTaskFromOIDCTokenRequest(ctx *context.Base) (*actions_model.ActionTask, error) {
	authHeader := ctx.Req.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, errors.New("bad authorization header")
	}

	taskID, err := actions_service.ParseAuthorizationToken(ctx.Req)
	if err != nil {
		log.Error("Error parsing authorization token: %v", err)
		return nil, errors.New("invalid authorization token")
	}
	if taskID == 0 {
		return nil, errors.New("invalid authorization token")
	}

	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Error("Error runner api getting task by ID: %v", err)
		return nil, errors.New("error runner api getting task")
	}
	if task.Status != actions_model.StatusRunning {
		return nil, errors.New("task is not running")
	}
	return task, nil
}

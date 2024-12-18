// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"

	"code.gitea.io/gitea/modules/setting"
)

type GenerateTokenRequest struct {
	Scope    string
	PutToken string
}

// GenerateActionsRunnerToken calls the internal GenerateActionsRunnerToken function
func GenerateActionsRunnerToken(ctx context.Context, scope string) (*ResponseText, ResponseExtra) {
	reqURL := setting.LocalURL + "api/internal/actions/generate_actions_runner_token"

	req := newInternalRequest(ctx, reqURL, "POST", GenerateTokenRequest{
		Scope: scope,
	})

	return requestJSONResp(req, &ResponseText{})
}

type SetTokenRequest struct {
	Scope string
	Token string
}

// SetActionsRunnerToken calls the internal GenerateActionsRunnerToken function
func SetActionsRunnerToken(ctx context.Context, scope, token string) (*ResponseText, ResponseExtra) {
	reqURL := setting.LocalURL + "api/internal/actions/set_actions_runner_token"

	req := newInternalRequest(ctx, reqURL, "POST", SetTokenRequest{
		Scope: scope,
		Token: token,
	})

	return requestJSONResp(req, &ResponseText{})
}

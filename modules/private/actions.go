// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"

	"code.gitea.io/gitea/modules/setting"
)

type GenerateTokenRequest struct {
	Scope string
}

// GenerateActionsRunnerToken calls the internal GenerateActionsRunnerToken function
func GenerateActionsRunnerToken(ctx context.Context, scope string) (string, ResponseExtra) {
	reqURL := setting.LocalURL + "api/internal/actions/generate_actions_runner_token"

	req := newInternalRequest(ctx, reqURL, "POST", GenerateTokenRequest{
		Scope: scope,
	})

	resp, extra := requestJSONResp(req, &responseText{})
	return resp.Text, extra
}

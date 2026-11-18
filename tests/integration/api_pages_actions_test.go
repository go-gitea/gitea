// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	"gitea.dev/tests"
)

func TestPagesActionsDeploymentAPI(t *testing.T) {
	defer tests.PrepareTestEnv(t, 1)()
	const taskToken = "8061e833a55f6fc0157c98b883e91fcfeeb1a71a"
	deploymentURL := "/api/v1/repos/user5/repo4/pages/deployments/"

	MakeRequest(t, NewRequest(t, "GET", deploymentURL+"invalid").AddTokenAuth(taskToken), http.StatusBadRequest)
	MakeRequest(t, NewRequest(t, "POST", deploymentURL+strings.Repeat("a", 64)+"/cancel").AddTokenAuth(taskToken), http.StatusNoContent)
}

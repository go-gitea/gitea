// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app_model "code.gitea.io/gitea/models/application"
)

func TestGiteaAppJWTAuth(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	app, err := app_model.GetAppByClientID(t.Context(), "b4732d48-a71d-4f80-9d9d-82dcfd77049c")
	assert.NoError(t, err)

	key, err := app_model.GenerateKeyPair()
	assert.NoError(t, err)
	assert.NotEmpty(t, key)

	pubKey, err := key.PublicKeyPEM()
	assert.NoError(t, err)

	err = app.AddJWTPublicKey(t.Context(), pubKey)
	assert.NoError(t, err)

	jwtStr, err := app_model.CreateJWTToken(key.PrivateKey, app.AppExternalData().ClientID, 600)
	assert.NoError(t, err)
	assert.NotEmpty(t, jwtStr)

	userReq := NewRequest(t, "GET", "/api/v1/user")
	userReq.SetHeader("Authorization", "Bearer "+jwtStr)
	userResp := MakeRequest(t, userReq, http.StatusOK)

	type userResponse struct {
		Login string `json:"login"`
		Email string `json:"email"`
	}

	userParsed := new(userResponse)
	require.NoError(t, json.Unmarshal(userResp.Body.Bytes(), userParsed))
	assert.Equal(t, "App1", userParsed.Login)
}

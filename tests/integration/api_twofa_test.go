// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
)

func TestAPITwoFactor(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 16})

	req := NewRequestf(t, "GET", "/api/v1/user")
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusOK)

	otpKey, err := totp.Generate(totp.GenerateOpts{
		SecretSize:  40,
		Issuer:      "gitea-test",
		AccountName: user.Name,
	})
	assert.NoError(t, err)

	tfa := &auth_model.TwoFactor{
		UID: user.ID,
	}
	assert.NoError(t, tfa.SetSecret(otpKey.Secret()))

	assert.NoError(t, auth_model.NewTwoFactor(tfa))

	req = NewRequestf(t, "GET", "/api/v1/user")
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusUnauthorized)

	passcode, err := totp.GenerateCode(otpKey.Secret(), time.Now())
	assert.NoError(t, err)

	req = NewRequestf(t, "GET", "/api/v1/user")
	req = AddBasicAuthHeader(req, user.Name)
	req.Header.Set("X-Gitea-OTP", passcode)
	MakeRequest(t, req, http.StatusOK)
}

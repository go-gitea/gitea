// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
)

func TestAPITwoFactor(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 16})

	req := NewRequest(t, "GET", "/api/v1/user").
		AddBasicAuth(user.Name)
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

	assert.NoError(t, auth_model.NewTwoFactor(db.DefaultContext, tfa))

	req = NewRequest(t, "GET", "/api/v1/user").
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusUnauthorized)

	passcode, err := totp.GenerateCode(otpKey.Secret(), time.Now())
	assert.NoError(t, err)

	req = NewRequest(t, "GET", "/api/v1/user").
		AddBasicAuth(user.Name)
	req.Header.Set("X-Gitea-OTP", passcode)
	MakeRequest(t, req, http.StatusOK)
}

func TestBasicAuthWithWebAuthn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user1 has no webauthn enrolled, he can request API with basic auth
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	unittest.AssertNotExistsBean(t, &auth_model.WebAuthnCredential{UserID: user1.ID})
	req := NewRequest(t, "GET", "/api/v1/user")
	req.SetBasicAuth(user1.Name, "password")
	MakeRequest(t, req, http.StatusOK)

	// user1 has no webauthn enrolled, he can request git protocol with basic auth
	req = NewRequest(t, "GET", "/user2/repo1/info/refs")
	req.SetBasicAuth(user1.Name, "password")
	MakeRequest(t, req, http.StatusOK)

	// user1 has no webauthn enrolled, he can request container package with basic auth
	req = NewRequest(t, "GET", "/v2/token")
	req.SetBasicAuth(user1.Name, "password")
	resp := MakeRequest(t, req, http.StatusOK)

	type tokenResponse struct {
		Token string `json:"token"`
	}
	var tokenParsed tokenResponse
	DecodeJSON(t, resp, &tokenParsed)
	assert.NotEmpty(t, tokenParsed.Token)

	// user32 has webauthn enrolled, he can't request API with basic auth
	user32 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 32})
	unittest.AssertExistsAndLoadBean(t, &auth_model.WebAuthnCredential{UserID: user32.ID})

	req = NewRequest(t, "GET", "/api/v1/user")
	req.SetBasicAuth(user32.Name, "notpassword")
	resp = MakeRequest(t, req, http.StatusUnauthorized)

	type userResponse struct {
		Message string `json:"message"`
	}
	var userParsed userResponse
	DecodeJSON(t, resp, &userParsed)
	assert.EqualValues(t, "Basic authorization is not allowed while webAuthn enrolled", userParsed.Message)

	// user32 has webauthn enrolled, he can't request git protocol with basic auth
	req = NewRequest(t, "GET", "/user2/repo1/info/refs")
	req.SetBasicAuth(user32.Name, "notpassword")
	MakeRequest(t, req, http.StatusUnauthorized)

	// user32 has webauthn enrolled, he can't request container package with basic auth
	req = NewRequest(t, "GET", "/v2/token")
	req.SetBasicAuth(user1.Name, "notpassword")
	MakeRequest(t, req, http.StatusUnauthorized)
}

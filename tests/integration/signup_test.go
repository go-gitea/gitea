// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSignup(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.Service.EnableCaptcha = false

	req := NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"user_name": "exampleUser",
		"email":     "exampleUser@example.com",
		"password":  "examplePassword!1",
		"retype":    "examplePassword!1",
	})
	MakeRequest(t, req, http.StatusSeeOther)

	// should be able to view new user's page
	req = NewRequest(t, "GET", "/exampleUser")
	MakeRequest(t, req, http.StatusOK)
}

func TestSignupAsRestricted(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.Service.EnableCaptcha = false
	setting.Service.DefaultUserIsRestricted = true

	req := NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"user_name": "restrictedUser",
		"email":     "restrictedUser@example.com",
		"password":  "examplePassword!1",
		"retype":    "examplePassword!1",
	})
	MakeRequest(t, req, http.StatusSeeOther)

	// should be able to view new user's page
	req = NewRequest(t, "GET", "/restrictedUser")
	MakeRequest(t, req, http.StatusOK)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "restrictedUser"})
	assert.True(t, user2.IsRestricted)
}

func TestSignupEmailValidation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	setting.Service.EnableCaptcha = false

	tests := []struct {
		email      string
		wantStatus int
		wantMsg    string
	}{
		{"exampleUser@example.com\r\n", http.StatusOK, translation.NewLocale("en-US").TrString("form.email_invalid")},
		{"exampleUser@example.com\r", http.StatusOK, translation.NewLocale("en-US").TrString("form.email_invalid")},
		{"exampleUser@example.com\n", http.StatusOK, translation.NewLocale("en-US").TrString("form.email_invalid")},
		{"exampleUser@example.com", http.StatusSeeOther, ""},
	}

	for i, test := range tests {
		req := NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
			"user_name": fmt.Sprintf("exampleUser%d", i),
			"email":     test.email,
			"password":  "examplePassword!1",
			"retype":    "examplePassword!1",
		})
		resp := MakeRequest(t, req, test.wantStatus)
		if test.wantMsg != "" {
			htmlDoc := NewHTMLParser(t, resp.Body)
			assert.Equal(t,
				test.wantMsg,
				strings.TrimSpace(htmlDoc.doc.Find(".ui.message").Text()),
			)
		}
	}
}

func TestSignupEmailActive(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.RegisterEmailConfirm, true)()

	// try to sign up and send the activation email
	req := NewRequestWithValues(t, "POST", "/user/sign_up", map[string]string{
		"user_name": "Test-User-1",
		"email":     "EmAiL-1@example.com",
		"password":  "password1",
		"retype":    "password1",
	})
	resp := MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `A new confirmation email has been sent to <b>EmAiL-1@example.com</b>.`)

	// access "user/activate" means trying to re-send the activation email
	session := loginUserWithPassword(t, "test-user-1", "password1")
	resp = session.MakeRequest(t, NewRequest(t, "GET", "/user/activate"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), "You have already requested an activation email recently")

	// access anywhere else will see an "Activate Your Account" prompt, and there is a chance to change email
	resp = session.MakeRequest(t, NewRequest(t, "GET", "/user/issues"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<input id="change-email" name="change_email" `)

	// post to "user/activate" with a new email
	session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/activate", map[string]string{"change_email": "email-changed@example.com"}), http.StatusSeeOther)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "Test-User-1"})
	assert.Equal(t, "email-changed@example.com", user.Email)
	email := unittest.AssertExistsAndLoadBean(t, &user_model.EmailAddress{Email: "email-changed@example.com"})
	assert.False(t, email.IsActivated)
	assert.True(t, email.IsPrimary)

	// generate an activation code from lower-cased email
	activationCode := user_model.GenerateUserTimeLimitCode(&user_model.TimeLimitCodeOptions{Purpose: user_model.TimeLimitCodeActivateAccount}, user)
	// and update the user email to case-sensitive, it shouldn't affect the verification later
	_, _ = db.Exec(db.DefaultContext, "UPDATE `user` SET email=? WHERE id=?", "EmAiL-changed@example.com", user.ID)
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "Test-User-1"})
	assert.Equal(t, "EmAiL-changed@example.com", user.Email)

	// access "user/activate" with a valid activation code, then get the "verify password" page
	resp = session.MakeRequest(t, NewRequest(t, "GET", "/user/activate?code="+activationCode), http.StatusOK)
	assert.Contains(t, resp.Body.String(), `<input id="verify-password"`)

	// try to use a wrong password, it should fail
	req = NewRequestWithValues(t, "POST", "/user/activate", map[string]string{
		"code":     activationCode,
		"password": "password-wrong",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), `Your password does not match`)
	assert.Contains(t, resp.Body.String(), `<input id="verify-password"`)
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "Test-User-1"})
	assert.False(t, user.IsActive)

	// then use a correct password, the user should be activated
	req = NewRequestWithValues(t, "POST", "/user/activate", map[string]string{
		"code":     activationCode,
		"password": "password1",
	})
	resp = session.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, "/", test.RedirectURL(resp))
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "Test-User-1"})
	assert.True(t, user.IsActive)
}

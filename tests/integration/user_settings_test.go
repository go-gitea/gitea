// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/translation"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestUserSettings(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("RevokeOAuth2GrantOfOtherUser", testRevokeOAuth2GrantOfOtherUser)
	t.Run("Account", testUserSettingsAccount)
	t.Run("UpdatePassword", testUserSettingsUpdatePassword)
	t.Run("UpdateEmail", testUserSettingsUpdateEmail)
	t.Run("DeleteEmail", testUserSettingsDeleteEmail)
	t.Run("Delete", testUserSettingsDelete)
	t.Run("Pages", testUserSettingsPages)
	t.Run("Security", testUserSettingsSecurity)
	t.Run("Applications", testUserSettingsApplications)
	t.Run("Keys", testUserSettingsKeys)
	t.Run("Secrets", testUserSettingsSecrets)
	t.Run("RenameInvalidUsername", testRenameInvalidUsername)
	t.Run("RenameReservedUsername", testRenameReservedUsername)
	t.Run("ChangeFullName", testChangeFullName)
	t.Run("LocationMapLink", testUserLocationMapLink)
	t.Run("RenameUsername", testRenameUsername)
}

// Validate that each navbar setting is correct. This checks that the
// appropriate context is passed everywhere the navbar is rendered
func assertNavbar(t *testing.T, doc *HTMLDoc) {
	// Only show the account page if users can change their email notifications, delete themselves, or manage credentials
	if setting.Admin.UserDisabledFeatures.Contains(setting.UserFeatureDeletion, setting.UserFeatureManageCredentials) && !setting.Service.EnableNotifyMail {
		AssertHTMLElement(t, doc, ".menu a[href='/user/settings/account']", false)
	} else {
		AssertHTMLElement(t, doc, ".menu a[href='/user/settings/account']", true)
	}

	if setting.Admin.UserDisabledFeatures.Contains(setting.UserFeatureManageMFA, setting.UserFeatureManageCredentials) {
		AssertHTMLElement(t, doc, ".menu a[href='/user/settings/security']", false)
	} else {
		AssertHTMLElement(t, doc, ".menu a[href='/user/settings/security']", true)
	}

	if setting.Admin.UserDisabledFeatures.Contains(setting.UserFeatureManageSSHKeys, setting.UserFeatureManageGPGKeys) {
		AssertHTMLElement(t, doc, ".menu a[href='/user/settings/keys']", false)
	} else {
		AssertHTMLElement(t, doc, ".menu a[href='/user/settings/keys']", true)
	}
}

func WithDisabledFeatures(t *testing.T, features ...string) {
	t.Helper()

	global := setting.Admin.UserDisabledFeatures
	user := setting.Admin.ExternalUserDisableFeatures

	setting.Admin.UserDisabledFeatures = container.SetOf(features...)
	setting.Admin.ExternalUserDisableFeatures = setting.Admin.UserDisabledFeatures

	t.Cleanup(func() {
		setting.Admin.UserDisabledFeatures = global
		setting.Admin.ExternalUserDisableFeatures = user
	})
}

func testRevokeOAuth2GrantOfOtherUser(t *testing.T) {
	// grant 1 belongs to user1, so user2 must not be able to act on it
	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "/user/settings/applications/oauth2/1/revoke/1", nil)
	session.MakeRequest(t, req, http.StatusNotFound)

	grant := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ID: 1})
	assert.EqualValues(t, 1, grant.UserID)
}

func testUserSettingsAccount(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		features       []string
		credentials    bool
		deleteSelector string
		deletion       bool
	}{
		{"all features enabled", nil, true, `form[action="/user/settings/account/delete"]`, true},
		{"credentials disabled", []string{setting.UserFeatureManageCredentials}, false, `form[action="/user/settings/account/delete"]`, true},
		{"deletion disabled", []string{setting.UserFeatureDeletion}, true, "#delete-form", false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			if len(testCase.features) > 0 {
				WithDisabledFeatures(t, testCase.features...)
			}

			session := loginUser(t, "user2")
			req := NewRequest(t, "GET", "/user/settings/account")
			resp := session.MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)

			if len(testCase.features) == 0 {
				// account navbar should display
				AssertHTMLElement(t, doc, ".menu a[href='/user/settings/account']", true)
			} else {
				assertNavbar(t, doc)
			}

			AssertHTMLElement(t, doc, "#password", testCase.credentials)
			AssertHTMLElement(t, doc, "#email", testCase.credentials)
			AssertHTMLElement(t, doc, testCase.deleteSelector, testCase.deletion)
		})
	}

	t.Run("deletion, credentials and email notifications are disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		mail := setting.Service.EnableNotifyMail
		setting.Service.EnableNotifyMail = false
		defer func() {
			setting.Service.EnableNotifyMail = mail
		}()

		WithDisabledFeatures(t, setting.UserFeatureDeletion, setting.UserFeatureManageCredentials)

		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user/settings/account")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}

func testUserSettingsUpdatePassword(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user2")
		req := NewRequestWithValues(t, "POST", "/user/settings/account", map[string]string{
			"old_password": "password",
			"password":     "password",
			"retype":       "password",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
	})

	t.Run("credentials disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		WithDisabledFeatures(t, setting.UserFeatureManageCredentials)
		session := loginUser(t, "user2")
		req := NewRequest(t, "POST", "/user/settings/account")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}

func testUserSettingsUpdateEmail(t *testing.T) {
	t.Run("credentials disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		WithDisabledFeatures(t, setting.UserFeatureManageCredentials)
		session := loginUser(t, "user2")
		req := NewRequest(t, "POST", "/user/settings/account/email")
		session.MakeRequest(t, req, http.StatusNotFound)
	})

	for _, testCase := range []struct {
		name string
		id   string
	}{
		{"primary email not found", "9999"},
		{"primary email not owned by user", "6"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			session := loginUser(t, "user2")
			req := NewRequestWithValues(t, "POST", "/user/settings/account/email", map[string]string{
				"_method": "PRIMARY",
				"id":      testCase.id,
			})
			resp := session.MakeRequest(t, req, http.StatusSeeOther)
			assert.Equal(t, "/user/settings/account", resp.Header().Get("Location"))
			flashMsg := session.GetCookieFlashMessage()
			assert.Equal(t, "The selected email address could not be found.", flashMsg.ErrorMsg)
		})
	}
}

func testUserSettingsDeleteEmail(t *testing.T) {
	t.Run("credentials disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		WithDisabledFeatures(t, setting.UserFeatureManageCredentials)
		session := loginUser(t, "user2")
		req := NewRequest(t, "POST", "/user/settings/account/email/delete")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}

func testUserSettingsDelete(t *testing.T) {
	t.Run("deletion disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		WithDisabledFeatures(t, setting.UserFeatureDeletion)

		session := loginUser(t, "user2")
		req := NewRequest(t, "POST", "/user/settings/account/delete")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}

func testUserSettingsPages(t *testing.T) {
	for _, page := range []string{"appearance", "packages", "packages/rules/add", "organization", "repos", "blocked_users"} {
		t.Run(page, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			session := loginUser(t, "user2")
			req := NewRequest(t, "GET", "/user/settings/"+page)
			resp := session.MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)

			assertNavbar(t, doc)
		})
	}
}

func testUserSettingsSecurity(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		feature  string
		register bool
	}{
		{"credentials disabled", setting.UserFeatureManageCredentials, true},
		{"mfa disabled", setting.UserFeatureManageMFA, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			WithDisabledFeatures(t, testCase.feature)

			session := loginUser(t, "user2")
			req := NewRequest(t, "GET", "/user/settings/security")
			resp := session.MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)

			assertNavbar(t, doc)

			AssertHTMLElement(t, doc, "#register-webauthn", testCase.register)
		})
	}

	t.Run("credentials and mfa disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		WithDisabledFeatures(t, setting.UserFeatureManageCredentials, setting.UserFeatureManageMFA)

		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user/settings/security")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}

func testUserSettingsApplications(t *testing.T) {
	t.Run("Applications", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user/settings/applications")
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		assertNavbar(t, doc)
	})

	t.Run("RegenerateAccessToken", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user2")

		before := unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{ID: 3, UID: 2})

		req := NewRequestWithValues(t, "POST", "/user/settings/applications/regenerate", map[string]string{
			"id": "3",
		})
		session.MakeRequest(t, req, http.StatusOK)

		after := unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{ID: 3, UID: 2})
		assert.Equal(t, before.Name, after.Name)
		assert.Equal(t, before.Scope, after.Scope)
		assert.NotEqual(t, before.TokenHash, after.TokenHash)
		assert.NotEqual(t, before.TokenSalt, after.TokenSalt)
	})

	t.Run("OAuth2", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		session := loginUser(t, "user2")

		t.Run("OAuth2ApplicationShow", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", "/user/settings/applications/oauth2/2")
			resp := session.MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)

			assertNavbar(t, doc)
		})

		t.Run("OAuthApplicationsEdit", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			t.Run("Invalid URL", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequestWithValues(t, "POST", "/user/settings/applications/oauth2/2", map[string]string{
					"application_name":    "Test native app",
					"redirect_uris":       "ftp://127.0.0.1",
					"confidential_client": "false",
				})
				resp := session.MakeRequest(t, req, http.StatusOK)
				doc := NewHTMLParser(t, resp.Body)
				msg := strings.TrimSpace(doc.Find(".ui.message.flash-message").Text())
				assert.Equal(t, `RedirectURIs: "ftp://127.0.0.1" is not a valid URL.`, msg)
			})

			t.Run("OK", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				defer test.MockVariableValue(&setting.OAuth2.CustomSchemes, []string{"my-app"})()
				for _, redirectURI := range []string{"http://127.0.0.1", "my-app://127.0.0.1"} {
					req := NewRequestWithValues(t, "POST", "/user/settings/applications/oauth2/2", map[string]string{
						"application_name":    "Test native app",
						"redirect_uris":       redirectURI,
						"confidential_client": "false",
					})
					session.MakeRequest(t, req, http.StatusSeeOther)
				}
			})
		})
	})
}

func testUserSettingsKeys(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		feature string
		ssh     bool
		gpg     bool
	}{
		{"all enabled", "", true, true},
		{"ssh keys disabled", setting.UserFeatureManageSSHKeys, false, true},
		{"gpg keys disabled", setting.UserFeatureManageGPGKeys, true, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			if testCase.feature != "" {
				WithDisabledFeatures(t, testCase.feature)
			}

			session := loginUser(t, "user2")
			req := NewRequest(t, "GET", "/user/settings/keys")
			resp := session.MakeRequest(t, req, http.StatusOK)
			doc := NewHTMLParser(t, resp.Body)

			assertNavbar(t, doc)

			AssertHTMLElement(t, doc, "#add-ssh-button", testCase.ssh)
			AssertHTMLElement(t, doc, "#add-gpg-key-panel", testCase.gpg)
		})
	}

	t.Run("ssh & gpg keys disabled", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		WithDisabledFeatures(t, setting.UserFeatureManageSSHKeys, setting.UserFeatureManageGPGKeys)

		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user/settings/keys")
		_ = session.MakeRequest(t, req, http.StatusNotFound)
	})
}

func testUserSettingsSecrets(t *testing.T) {
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user/settings/actions/secrets")
	if setting.Actions.Enabled {
		resp := session.MakeRequest(t, req, http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)

		assertNavbar(t, doc)
	} else {
		session.MakeRequest(t, req, http.StatusNotFound)
	}
}

func testRenameInvalidUsername(t *testing.T) {
	invalidUsernames := []string{
		"%2f*",
		"%2f.",
		"%2f..",
		"%00",
		"thisHas ASpace",
		"p<A>tho>lo<gical",
		".",
		"..",
		".well-known",
		".abc",
		"abc.",
		"a..bc",
		"a...bc",
		"a.-bc",
		"a._bc",
		"a_-bc",
		"a/bc",
		"☁️",
		"-",
		"--diff",
		"-im-here",
		"a space",
	}

	session := loginUser(t, "user2")
	for _, invalidUsername := range invalidUsernames {
		t.Logf("Testing username %s", invalidUsername)

		req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"name":  invalidUsername,
			"email": "user2@example.com",
		})
		resp := session.MakeRequest(t, req, http.StatusBadRequest)
		data := DecodeJSON(t, resp, map[string]any{})
		assert.Equal(t, []any{"Name"}, data["errorFields"])
		assert.Contains(t,
			data["errorMessage"],
			translation.NewLocale("en-US").TrString("form.username_error", "Name"),
		)

		unittest.AssertNotExistsBean(t, &user_model.User{Name: invalidUsername})
	}
}

func testRenameReservedUsername(t *testing.T) {
	// ".", "..", ".well-known" are also reserved but invalid as form input.
	reservedNames := []string{
		"api",
		"openapi3.v1.json",
		"swagger.v1.json",
	}
	patternNotAllowedNames := []string{
		"name.keys",
	}

	session := loginUser(t, "user2")
	locale := translation.NewLocale("en-US")
	check := func(name, msgKey string) {
		req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"name":     name,
			"email":    "user2@example.com",
			"language": "en-US",
		})
		resp := session.MakeRequest(t, req, http.StatusBadRequest)
		data := DecodeJSON(t, resp, map[string]any{})
		assert.Equal(t, locale.TrString(msgKey, name), data["errorMessage"])
		assert.Equal(t, []any{"Name"}, data["errorFields"])
		unittest.AssertNotExistsBean(t, &user_model.User{Name: name})
	}
	for _, name := range reservedNames {
		check(name, "user.form.name_reserved")
	}
	for _, name := range patternNotAllowedNames {
		check(name, "user.form.name_pattern_not_allowed")
	}
}

func testChangeFullName(t *testing.T) {
	session := loginUser(t, "user2")
	postSettings := func(t *testing.T, values map[string]string, expectedStatus int) map[string]any {
		t.Helper()
		resp := session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/settings", values), expectedStatus)
		return DecodeJSON(t, resp, map[string]any{})
	}

	t.Run("Change", func(t *testing.T) {
		postSettings(t, map[string]string{"full_name": "New Name"}, http.StatusOK)
		unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2", FullName: "New Name"})
	})

	t.Run("Disabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Admin.UserDisabledFeatures, container.SetOf(setting.UserFeatureChangeFullName))()
		postSettings(t, map[string]string{"location": "Elsewhere"}, http.StatusOK)
		unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2", FullName: "New Name", Location: "Elsewhere"})

		data := postSettings(t, map[string]string{"full_name": "Other Name"}, http.StatusBadRequest)
		assert.Equal(t, []any{"FullName"}, data["errorFields"])
		assert.Equal(t, "Changing full name is disabled.", data["errorMessage"])
		unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2", FullName: "New Name"})
	})

	t.Run("Clear", func(t *testing.T) {
		postSettings(t, map[string]string{"full_name": ""}, http.StatusOK)
		assert.Empty(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"}).FullName)
	})
}

func testRenameUsername(t *testing.T) {
	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"name":     "newUsername",
		"email":    "user2@example.com",
		"language": "en-US",
	})
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "/user/settings", DecodeJSON(t, resp, map[string]any{})["redirect"])

	unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "newUsername"})
	unittest.AssertNotExistsBean(t, &user_model.User{Name: "user2"})
}

func testUserLocationMapLink(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.UserLocationMapURL, "https://example/foo/")()

	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"name":     "user2",
		"email":    "user@example.com",
		"language": "en-US",
		"location": "A/b",
	})
	session.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user2/")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	AssertHTMLElement(t, htmlDoc, `a[href="https://example/foo/A%2Fb"]`, true)
}

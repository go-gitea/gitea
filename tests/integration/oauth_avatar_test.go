// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/tests"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pngBytes returns a tiny but valid PNG payload for avatar tests.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// avatarTestServer returns an httptest.Server serving a tiny PNG at /avatar.png,
// records every request to *requests, and optionally enforces a non-empty
// "Gitea " User-Agent. When requireGiteaUA is true the server returns 403 to
// any request without a User-Agent starting with "Gitea " (mirroring real-world
// hosts such as upload.wikimedia.org).
func avatarTestServer(t *testing.T, requireGiteaUA bool) (srv *httptest.Server, requests *atomic.Int32, lastUA *atomic.Value) {
	t.Helper()
	requests = &atomic.Int32{}
	lastUA = &atomic.Value{}
	body := pngBytes(t)
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		ua := r.Header.Get("User-Agent")
		lastUA.Store(ua)
		if r.URL.Path != "/avatar.png" {
			http.NotFound(w, r)
			return
		}
		if requireGiteaUA && !strings.HasPrefix(ua, "Gitea ") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
	return srv, requests, lastUA
}

// triggerOAuth2AutoRegisterLogin performs a single OAuth2 callback with
// EnableAutoRegistration=true and a mocked goth.User. It is the closest
// integration-test reproduction of a real "first OIDC sign-in".
func triggerOAuth2AutoRegisterLogin(t *testing.T, sourceName, providerName string, gothUser goth.User) *http.Response {
	t.Helper()
	defer test.MockVariableValue(&setting.OAuth2Client.Username, "")()
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
	defer test.MockVariableValue(&gothic.CompleteUserAuth, func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
		gothUser.Provider = providerName
		return gothUser, nil
	})()

	session := emptyTestSession(t)
	req := NewRequest(t, "GET", "/user/oauth2/"+sourceName+"/callback?code=XYZ&state=XYZ")
	resp := session.MakeRequest(t, req, http.StatusSeeOther)
	return resp.Result()
}

// TestOAuth2AvatarFromPicture verifies the OIDC `picture` claim becomes the
// user's avatar through the OAuth2 sign-in path, including:
//   - happy path (auto-register flow) downloads & uploads the avatar
//   - the request carries the "Gitea <version>" User-Agent (issue: hosts that
//     reject Go's default UA, e.g. upload.wikimedia.org, returned 403)
//   - link-account "register" flow ALSO syncs the avatar (previously missing,
//     leaving first-time OIDC users with an identicon)
//   - non-200 / empty URL / UPDATE_AVATAR=false paths are no-ops
func TestOAuth2AvatarFromPicture(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	mockServer := createOAuth2MockProvider()
	defer mockServer.Close()

	addOAuth2Source(t, "test-oidc-avatar", oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      "test-client-id",
		OpenIDConnectAutoDiscoveryURL: mockServer.URL + "/.well-known/openid-configuration",
	})
	authSource, err := auth_model.GetActiveOAuth2SourceByAuthName(t.Context(), "test-oidc-avatar")
	require.NoError(t, err)
	providerName := authSource.Cfg.(*oauth2.Source).Provider

	t.Run("AutoRegister_FetchesAvatarFromPictureWithGiteaUA", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.UpdateAvatar, true)()

		// requireGiteaUA=true mirrors hosts like upload.wikimedia.org which
		// reject Go's default User-Agent. Without the "Gitea " UA fix this
		// returns 403 and the avatar silently fails to update.
		avatarSrv, requests, lastUA := avatarTestServer(t, true)
		defer avatarSrv.Close()

		gothUser := goth.User{
			UserID:    "oidc-user-ua-pic",
			Email:     "oidc-user-ua-pic@example.com",
			Name:      "OIDC UA Pic",
			AvatarURL: avatarSrv.URL + "/avatar.png",
		}
		triggerOAuth2AutoRegisterLogin(t, "test-oidc-avatar", providerName, gothUser)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "oidc-user-ua-pic"})
		assert.True(t, user.UseCustomAvatar, "use_custom_avatar must be set after a successful avatar download")
		assert.NotEmpty(t, user.Avatar, "avatar hash must be set")
		assert.Equal(t, int32(1), requests.Load(), "exactly one avatar fetch should happen")
		ua, _ := lastUA.Load().(string)
		assert.True(t, strings.HasPrefix(ua, "Gitea "), "User-Agent must start with 'Gitea ' (got %q)", ua)
	})

	t.Run("AutoRegister_NonOK_DoesNotUpdateAvatar", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.UpdateAvatar, true)()

		failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "no", http.StatusForbidden)
		}))
		defer failSrv.Close()

		gothUser := goth.User{
			UserID:    "oidc-user-non200",
			Email:     "oidc-user-non200@example.com",
			Name:      "OIDC NonOK",
			AvatarURL: failSrv.URL + "/avatar.png",
		}
		triggerOAuth2AutoRegisterLogin(t, "test-oidc-avatar", providerName, gothUser)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "oidc-user-non200"})
		assert.False(t, user.UseCustomAvatar, "non-200 must not flip use_custom_avatar")
		assert.Empty(t, user.Avatar, "non-200 must not set the avatar hash")
	})

	t.Run("AutoRegister_EmptyPicture_NoFetch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.UpdateAvatar, true)()

		avatarSrv, requests, _ := avatarTestServer(t, false)
		defer avatarSrv.Close()

		gothUser := goth.User{
			UserID: "oidc-user-no-pic",
			Email:  "oidc-user-no-pic@example.com",
			Name:   "OIDC No Pic",
			// AvatarURL intentionally empty
		}
		triggerOAuth2AutoRegisterLogin(t, "test-oidc-avatar", providerName, gothUser)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "oidc-user-no-pic"})
		assert.False(t, user.UseCustomAvatar)
		assert.Empty(t, user.Avatar)
		assert.Equal(t, int32(0), requests.Load(), "no HTTP request should be made when picture URL is empty")
	})

	t.Run("AutoRegister_UpdateAvatarFalse_NoFetch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.UpdateAvatar, false)()

		avatarSrv, requests, _ := avatarTestServer(t, false)
		defer avatarSrv.Close()

		gothUser := goth.User{
			UserID:    "oidc-user-disabled",
			Email:     "oidc-user-disabled@example.com",
			Name:      "OIDC Disabled",
			AvatarURL: avatarSrv.URL + "/avatar.png",
		}
		triggerOAuth2AutoRegisterLogin(t, "test-oidc-avatar", providerName, gothUser)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "oidc-user-disabled"})
		assert.False(t, user.UseCustomAvatar)
		assert.Empty(t, user.Avatar)
		assert.Equal(t, int32(0), requests.Load(), "no HTTP request should be made when UPDATE_AVATAR is false")
	})

	t.Run("LinkAccountRegister_FetchesAvatarFromPicture", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.UpdateAvatar, true)()

		avatarSrv, requests, lastUA := avatarTestServer(t, true)
		defer avatarSrv.Close()

		const newUserName = "oidc-link-register"
		const newUserEmail = "oidc-link-register@example.com"

		// The link-account register page expects a goth.User in the session.
		// Mocking it via web.RouteMock avoids spinning up the full callback
		// flow while still exercising LinkAccountPostRegister end-to-end.
		injectLinkAccountData := func(ctx *context.Context) {
			require.NoError(t, auth.Oauth2SetLinkAccountData(ctx, auth.LinkAccountData{
				AuthSourceID: authSource.ID,
				GothUser: goth.User{
					Provider:  providerName,
					UserID:    "oidc-link-register-sub",
					Email:     newUserEmail,
					Name:      "OIDC Link Register",
					AvatarURL: avatarSrv.URL + "/avatar.png",
				},
			}))
		}
		defer web.RouteMockReset()
		web.RouteMock(web.MockAfterMiddlewares, injectLinkAccountData)

		session := emptyTestSession(t)
		req := NewRequestWithValues(t, "POST", "/user/link_account_signup", map[string]string{
			"user_name": newUserName,
			"email":     newUserEmail,
			"password":  "AVeryStrongPassword!1",
			"retype":    "AVeryStrongPassword!1",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: newUserName})
		require.Equal(t, auth_model.OAuth2, user.LoginType, "user should be created as OAuth2")
		assert.True(t, user.UseCustomAvatar, "register-link flow must sync avatar from `picture` claim")
		assert.NotEmpty(t, user.Avatar, "avatar hash must be set after register-link flow")
		assert.GreaterOrEqual(t, requests.Load(), int32(1), "avatar URL must be fetched at least once")
		ua, _ := lastUA.Load().(string)
		assert.True(t, strings.HasPrefix(ua, "Gitea "), "User-Agent must start with 'Gitea ' (got %q)", ua)
	})
}

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

// avatarTestServer serves a PNG at /avatar.png but returns 403 unless the
// request's User-Agent starts with "Gitea " (mirrors hosts like Wikimedia
// that reject Go's default UA). A successful avatar sync proves the UA fix.
func avatarTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	body := buf.Bytes()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/avatar.png" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Gitea ") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(body)
	}))
}

func TestOAuth2AvatarFromPicture(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.OAuth2Client.UpdateAvatar, true)()

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

	avatarSrv := avatarTestServer(t)
	defer avatarSrv.Close()

	t.Run("AutoRegister", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.Username, "")()
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
		defer test.MockVariableValue(&gothic.CompleteUserAuth, func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
			return goth.User{
				Provider:  providerName,
				UserID:    "oidc-user-ua-pic",
				Email:     "oidc-user-ua-pic@example.com",
				Name:      "OIDC UA Pic",
				AvatarURL: avatarSrv.URL + "/avatar.png",
			}, nil
		})()

		req := NewRequest(t, "GET", "/user/oauth2/test-oidc-avatar/callback?code=XYZ&state=XYZ")
		emptyTestSession(t).MakeRequest(t, req, http.StatusSeeOther)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LoginName: "oidc-user-ua-pic"})
		assert.True(t, user.UseCustomAvatar, "avatar must sync (requires Gitea UA)")
		assert.NotEmpty(t, user.Avatar)
	})

	t.Run("LinkAccountRegister", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		const newUserName = "oidc-link-register"
		const newUserEmail = "oidc-link-register@example.com"

		defer web.RouteMockReset()
		web.RouteMock(web.MockAfterMiddlewares, func(ctx *context.Context) {
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
		})

		req := NewRequestWithValues(t, "POST", "/user/link_account_signup", map[string]string{
			"user_name": newUserName,
			"email":     newUserEmail,
			"password":  "AVeryStrongPassword!1",
			"retype":    "AVeryStrongPassword!1",
		})
		emptyTestSession(t).MakeRequest(t, req, http.StatusSeeOther)

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: newUserName})
		require.Equal(t, auth_model.OAuth2, user.LoginType)
		assert.True(t, user.UseCustomAvatar, "register-link flow must sync avatar from `picture` claim")
		assert.NotEmpty(t, user.Avatar)
	})
}

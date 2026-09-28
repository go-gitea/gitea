// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/public"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/web/middleware"
	"gitea.dev/services/webtheme"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The built-in themes were renamed from "theme-gitea-*.css" to "theme-teabag-*.css",
// but users who picked a theme before the rename still have the old name in "user"."theme",
// and anonymous users still have it in the theme cookie, so both must keep resolving.
func TestWebThemeLegacyName(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	teabagDarkAsset := public.AssetURI("web_src/css/themes/theme-teabag-dark.css")

	setUserTheme := func(theme string) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user.Theme = theme
		require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "theme"))
	}

	assertRendersTeabagDark := func(t *testing.T, session *TestSession) {
		t.Helper()
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
		body := resp.Body.String()
		htmlDoc := NewHTMLParser(t, strings.NewReader(body))
		assert.Equal(t, "teabag-dark", htmlDoc.Find("html").AttrOr("data-theme", ""))
		assert.Contains(t, body, teabagDarkAsset)
	}

	t.Run("StoredUserTheme", func(t *testing.T) {
		setUserTheme("gitea-dark")
		assertRendersTeabagDark(t, loginUser(t, "user2"))

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		assert.Equal(t, "gitea-dark", user.Theme, "the stored value stays as is, only its resolution changes")
	})

	t.Run("ThemeCookie", func(t *testing.T) {
		setUserTheme("")
		session := emptyTestSession(t)
		session.MakeRequest(t, NewRequest(t, "POST", "/-/web-theme/apply?theme=gitea-dark"), http.StatusOK)
		assert.Equal(t, "gitea-dark", session.GetSiteCookie(middleware.CookieTheme))
		assertRendersTeabagDark(t, session)
	})

	t.Run("UpdateThemePost", func(t *testing.T) {
		session := loginUser(t, "user2")
		req := NewRequestWithValues(t, "POST", "/user/settings/appearance/theme", map[string]string{"theme": "gitea-dark"})
		session.MakeRequest(t, req, http.StatusSeeOther)
		assert.Empty(t, session.GetCookieFlashMessage().ErrorMsg)
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		assert.Equal(t, "gitea-dark", user.Theme)
	})

	t.Run("AppearancePage", func(t *testing.T) {
		setUserTheme("gitea-dark")
		session := loginUser(t, "user2")
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user/settings/appearance"), http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		// the dropdown only offers the renamed themes, so it has to be pre-selected by the new name
		selected := htmlDoc.Find(`form[action$="/user/settings/appearance/theme"] input[name="theme"]`).AttrOr("value", "")
		assert.Equal(t, "teabag-dark", selected)
	})
}

// Guards against shipping a DEFAULT_THEME that no theme file provides: every page falls back
// to it, so a name left behind by a theme rename silently unstyles the whole install.
func TestDefaultThemeResolves(t *testing.T) {
	info := webtheme.GetThemeMetaInfo(setting.UI.DefaultTheme)
	require.NotNil(t, info, "DEFAULT_THEME %q does not resolve to an available theme", setting.UI.DefaultTheme)
	assert.Equal(t, setting.UI.DefaultTheme, info.InternalName, "DEFAULT_THEME must name a theme directly, not a legacy alias")
}

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package install

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codenest.dev/models/unittest"
	"codenest.dev/modules/setting"
	"codenest.dev/services/contexttest"
	"codenest.dev/services/forms"

	"github.com/stretchr/testify/assert"
)

func TestRoutes(t *testing.T) {
	r := Routes()
	assert.NotNil(t, r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `class="page-content install"`)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/no-such", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 404, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/assets/img/codenest.svg", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestFillInstallConfig(t *testing.T) {
	ctx, _ := contexttest.MockContext(t, "/")
	t.Run("WithEnv", func(t *testing.T) {
		f := &forms.InstallForm{AppName: "TestAppName"}
		cfg := fillInstallConfig(ctx, []string{
			"CODENEST__OAUTH2__JWT_SECRET_URI=any",
			"CODENEST____APP_NAME=EnvAppName",
		}, f)
		assert.Equal(t, "EnvAppName", cfg.Section("").Key("APP_NAME").String())
		assert.Empty(t, cfg.Section("oauth2").Key("JWT_SECRET").String())
		assert.Equal(t, "any", cfg.Section("oauth2").Key("JWT_SECRET_URI").String())
	})
	t.Run("NoEnv", func(t *testing.T) {
		f := &forms.InstallForm{AppName: "TestAppName"}
		cfg := fillInstallConfig(ctx, []string{}, f)
		assert.Equal(t, "TestAppName", cfg.Section("").Key("APP_NAME").String())
		assert.NotEmpty(t, cfg.Section("oauth2").Key("JWT_SECRET").String())
		assert.Empty(t, cfg.Section("oauth2").Key("JWT_SECRET_URI").String())
	})
	t.Run("CleanUp", func(t *testing.T) {
		tmpFile := t.TempDir() + "/test.ini"
		f := &forms.InstallForm{AppName: "foo\x00\r\nbar"}
		cfg := fillInstallConfig(ctx, []string{}, f)
		err := cfg.SaveTo(tmpFile)
		assert.NoError(t, err)
		ini, err := setting.NewConfigProviderFromFile(tmpFile)
		assert.NoError(t, err)
		assert.Equal(t, `foo   bar`, ini.Section("").Key("APP_NAME").String())
	})
}

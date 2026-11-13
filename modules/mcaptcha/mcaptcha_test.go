// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcaptcha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestMCaptchaVerify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/pow/siteverify":
			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req["key"] == "test-site-key" {
				w.WriteHeader(http.StatusOK)
				assert.Equal(t, "test-secret", req["secret"])
				resp := map[string]bool{"valid": req["token"] == "token-valid"}
				_ = json.NewEncoder(w).Encode(resp)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		}
	}))
	defer srv.Close()

	defer test.MockVariableValue(&setting.Service)()
	setting.Service.McaptchaURL = strings.TrimSuffix(srv.URL, "/") + "/"
	setting.Service.McaptchaSitekey = "test-site-key"
	setting.Service.McaptchaSecret = "test-secret"
	valid, err := Verify(t.Context(), "token-valid")
	assert.NoError(t, err)
	assert.True(t, valid)
	valid, err = Verify(t.Context(), "token-invalid")
	assert.NoError(t, err)
	assert.False(t, valid)

	setting.Service.McaptchaSitekey = "test-site-key-invalid"
	valid, err = Verify(t.Context(), "token-invalid")
	assert.Error(t, err)
	assert.False(t, valid)
}

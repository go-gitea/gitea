// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/forms"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenIDAssociation(t *testing.T) {
	unittest.PrepareTestEnv(t)
	defer test.MockVariableValue(&setting.AppURL, "https://gitea.example/")()
	defer test.MockVariableValue(&setting.AppSubURL, "")()
	defer test.MockVariableValue(&setting.Security.AllowedHostList, "127.0.0.1:*")()

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			assert.Equal(t, "check_authentication", r.FormValue("openid.mode"))
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "ns:http://specs.openid.net/auth/2.0\nis_valid:true\n")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html><head><link rel="openid2.provider" href="http://%s/provider"></head></html>`, r.Host)
	}))
	defer provider.Close()
	identity := provider.URL + "/identity"
	ctx, resp := contexttest.MockContext(t, "POST /user/settings/security/openid")
	contexttest.LoadUser(t, ctx, 2)
	web.SetForm(ctx, &forms.AddOpenIDForm{Openid: identity})
	OpenIDPost(ctx)
	require.Equal(t, http.StatusSeeOther, resp.Code)
	redirect, err := url.Parse(resp.Header().Get("Location"))
	require.NoError(t, err)
	callback, err := url.Parse(redirect.Query().Get("openid.return_to"))
	require.NoError(t, err)
	state := callback.Query().Get("state")
	require.NotEmpty(t, state)

	params := url.Values{
		"openid.ns":             {"http://specs.openid.net/auth/2.0"},
		"openid.mode":           {"id_res"},
		"openid.op_endpoint":    {provider.URL + "/provider"},
		"openid.claimed_id":     {identity},
		"openid.identity":       {identity},
		"openid.return_to":      {callback.String()},
		"openid.response_nonce": {time.Now().UTC().Format("2006-01-02T15:04:05Z") + "association"},
		"openid.assoc_handle":   {"test-association"},
		"openid.signed":         {"op_endpoint,claimed_id,identity,return_to,response_nonce,assoc_handle"},
		"openid.sig":            {"test-signature"},
	}
	for _, tc := range []struct {
		name  string
		uid   int64
		state string
	}{
		{"unsolicited", 2, ""},
		{"wrong state", 2, "wrong"},
		{"other user", 1, state},
		{"expired", 2, base.CreateTimeLimitCode("openid_association:2", openIDAssociationCodeMinutes, time.Now().Add(-time.Hour), nil)},
		{"other purpose", 2, base.CreateTimeLimitCode("reset_password:2", openIDAssociationCodeMinutes, time.Now(), nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params.Set("state", tc.state)
			ctx, resp := contexttest.MockContext(t, "GET /user/settings/security?"+params.Encode())
			contexttest.LoadUser(t, ctx, tc.uid)
			Security(ctx)
			assert.Equal(t, http.StatusForbidden, resp.Code)
			unittest.AssertNotExistsBean(t, &user_model.UserOpenID{URI: identity})
		})
	}

	params.Set("state", state)
	ctx, resp = contexttest.MockContext(t, "GET /user/settings/security?"+params.Encode())
	contexttest.LoadUser(t, ctx, 2)
	Security(ctx)
	require.Equal(t, http.StatusSeeOther, resp.Code)
	unittest.AssertExistsAndLoadBean(t, &user_model.UserOpenID{UID: 2, URI: identity})

	ctx, resp = contexttest.MockContext(t, "GET /user/settings/security?"+params.Encode())
	contexttest.LoadUser(t, ctx, 2)
	Security(ctx)
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "Nonce already used", ctx.Flash.ErrorMsg)
}

func TestDeleteOpenIDReturnsNotFoundForOtherUsersAddress(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "POST /user/settings/security?id=1")
	contexttest.LoadUser(t, ctx, 2)
	DeleteOpenID(ctx)
	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}

func TestToggleOpenIDVisibilityReturnsNotFoundForOtherUsersAddress(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "POST /user/settings/security?id=1")
	contexttest.LoadUser(t, ctx, 2)
	ToggleOpenIDVisibility(ctx)
	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}

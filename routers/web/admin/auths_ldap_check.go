// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	"gitea.dev/models/auth"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

// TestLDAPConnection tests an LDAP configuration without saving it.
func TestLDAPConnection(ctx *context.Context) {
	form := *web.GetForm[*forms.AuthenticationForm](ctx)
	if ctx.HasError() {
		ctx.JSON(http.StatusBadRequest, map[string]any{"ok": false, "message": ctx.Locale.TrString("admin.auths.test_connection.invalid_form")})
		return
	}
	if form.Type != auth.LDAP.Int() && form.Type != auth.DLDAP.Int() {
		ctx.JSON(http.StatusBadRequest, map[string]any{"ok": false, "message": ctx.Locale.TrString("admin.auths.test_connection.not_ldap")})
		return
	}
	result := parseLDAPConfig(form).TestConnection(form.Type == auth.DLDAP.Int())
	result.Message = ctx.Locale.TrString(result.Message)
	result.TranslateDetails(func(key string) string { return ctx.Locale.TrString(key) })
	ctx.JSON(http.StatusOK, result)
}

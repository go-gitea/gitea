// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/services/context"
	"gitea.dev/services/oauth2_provider"
)

// OIDCWellKnown generates JSON so OIDC clients know Gitea's capabilities
func OIDCWellKnown(ctx *context.Context) {
	if !setting.OAuth2.Enabled {
		http.NotFound(ctx.Resp, ctx.Req)
		return
	}
	jwtRegisteredClaims := oauth2_provider.NewJwtRegisteredClaimsFromUser("well-known", 0, nil)
	oidcIssuer := jwtRegisteredClaims.Issuer // use the consistent issuer from the JWT registered claims
	oidcBaseUrl := strings.TrimSuffix(setting.AppURL, "/")

	m := map[string]any{
		"issuer":                 oidcIssuer,
		"authorization_endpoint": oidcBaseUrl + "/login/oauth/authorize",
		"token_endpoint":         oidcBaseUrl + "/login/oauth/access_token",
		"jwks_uri":               oidcBaseUrl + "/login/oauth/keys",
		"userinfo_endpoint":      oidcBaseUrl + "/login/oauth/userinfo",
		"introspection_endpoint": oidcBaseUrl + "/login/oauth/introspect",
		"response_types_supported": []string{
			"code",
			"id_token",
		},
		"id_token_signing_alg_values_supported": []string{
			oauth2_provider.DefaultSigningKey.SigningMethod().Alg(),
		},
		"subject_types_supported": []string{
			"public",
		},
		"scopes_supported": oauth2_provider.GeneralScopesSupported(),
		"claims_supported": []string{
			"aud",
			"exp",
			"iat",
			"iss",
			"sub",
			"name",
			"preferred_username",
			"profile",
			"picture",
			"website",
			"locale",
			"updated_at",
			"email",
			"email_verified",
			"groups",
		},
		"code_challenge_methods_supported": []string{
			"plain",
			"S256",
		},
		"grant_types_supported": []string{
			"authorization_code",
			"refresh_token",
		},
	}
	ctx.JSON(http.StatusOK, m)
}

// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"encoding/base64"
	"fmt"
	"net/url"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"

	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/openidConnect"
)

// EndSessionEndpoint returns end_session_endpoint URI for OIDC sources
func (source *Source) EndSessionEndpoint(ctx *context.Context) (string, string, error) {
	redirect := &url.URL{}
	state := ""
	providerName := source.authSource.Name

	gothProvider, err := goth.GetProvider(providerName)
	if err != nil {
		return "", "", err
	}

	oidcProvider, ok := gothProvider.(*openidConnect.Provider)
	if ok && oidcProvider.OpenIDConfig != nil && len(oidcProvider.OpenIDConfig.EndSessionEndpoint) > 0 {
		if redirect, err = url.Parse(oidcProvider.OpenIDConfig.EndSessionEndpoint); err != nil {
			return "", "", err
		}

		r, err := util.CryptoRandomBytes(8)
		if err != nil {
			return "", "", err
		}
		state = base64.RawURLEncoding.EncodeToString(r)

		values := url.Values{}
		values.Set("client_id", oidcProvider.ClientKey)
		values.Set("post_logout_redirect_uri", fmt.Sprintf("%suser/oauth2/%s/logout/callback", setting.AppURL, providerName))
		values.Set("state", state)

		if ctx.Doer != nil {
			t, err := auth_model.GetExternalAuthTokenBySessionID(ctx, ctx.Session.ID())
			if auth_model.IsErrExternalAuthTokenNotExist(err) {
				log.Error("EndSessionEndpoint: %v", err)
			} else if err != nil {
				return "", "", err
			} else if t.UserID == ctx.Doer.ID && len(t.IDToken) > 0 {
				values.Set("id_token_hint", t.IDToken)
			} else {
				log.Error("EndSessionEndpoint IDToken missing for UserID %d [SessionID: %s, Provider: %s]", ctx.Doer.ID, ctx.Session.ID(), providerName)
			}
		}

		redirect.RawQuery = values.Encode()
	}

	return redirect.String(), state, nil
}

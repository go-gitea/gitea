// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"fmt"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/util"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/openidConnect"
)

func getProviderForRequest(provider string, req *http.Request, source *Source) (goth.Provider, error) {
	gothProvidersLock.RLock()
	gothProvider, err := goth.GetProvider(provider)
	gothProvidersLock.RUnlock()
	if err != nil {
		return nil, err
	}

	if !source.AutoCallbackURL {
		return gothProvider, nil
	}

	callbackURL := httplib.GuessCurrentAppURL(req.Context()) + "user/oauth2/" + url.PathEscape(provider) + "/callback"
	if callbackURL == DefaultCallbackURL(provider) {
		return gothProvider, nil
	}

	if oidcProvider, ok := gothProvider.(*openidConnect.Provider); ok && oidcProvider.OpenIDConfig != nil {
		return cloneOIDCProviderWithCallbackURL(provider, callbackURL, source, oidcProvider.OpenIDConfig)
	}

	return createProviderWithCallbackURL(provider, callbackURL, source)
}

// Callout redirects request/response pair to authenticate against the provider
func (source *Source) Callout(req *http.Request, w http.ResponseWriter) error {
	// not sure whether preserving query string makes sense globally, keep consistency with existing code
	req.URL.RawQuery = "provider=" + url.QueryEscape(source.authSource.Name)

	p, err := getProviderForRequest(source.authSource.Name, req, source)
	if err != nil {
		return err
	}

	sess, err := p.BeginAuth(gothic.SetState(req))
	if err != nil {
		return err
	}

	authURL, err := sess.GetAuthURL()
	if err != nil {
		return err
	}

	if err := gothic.StoreInSession(p.Name(), sess.Marshal(), req, w); err != nil {
		return err
	}

	http.Redirect(w, req, authURL, http.StatusTemporaryRedirect)
	return nil
}

// Callback handles OAuth callback, resolves to a goth user andCompleteUserAuth
func (source *Source) Callback(req *http.Request, w http.ResponseWriter) (goth.User, error) {
	// not sure whether preserving query string makes sense globally, keep consistency with existing code
	req.URL.RawQuery += "&provider=" + url.QueryEscape(source.authSource.Name)

	p, err := getProviderForRequest(source.authSource.Name, req, source)
	if err != nil {
		return goth.User{}, err
	}

	value, err := gothic.GetFromSession(p.Name(), req)
	if err != nil {
		return goth.User{}, err
	}
	defer func() {
		_ = gothic.Logout(w, req)
	}()

	sess, err := p.UnmarshalSession(value)
	if err != nil {
		return goth.User{}, err
	}

	query := req.URL.Query()
	if errMsg := query.Get("error"); errMsg != "" {
		return goth.User{}, util.NewInvalidArgumentErrorf("oauth2 error: %s: %s", errMsg, query.Get("error_description"))
	}

	state := gothic.GetState(req)
	if len(state) > 0 && query.Get("state") != state {
		return goth.User{}, fmt.Errorf("state token mismatch")
	}

	user, err := p.FetchUser(sess)
	if err == nil {
		return user, nil
	}

	params := req.URL.Query()
	if params.Encode() == "" && req.Method == http.MethodPost {
		_ = req.ParseForm()
		params = req.Form
	}

	_, err = sess.Authorize(p, params)
	if err != nil {
		return goth.User{}, err
	}

	if err := gothic.StoreInSession(p.Name(), sess.Marshal(), req, w); err != nil {
		return goth.User{}, err
	}

	return p.FetchUser(sess)
}

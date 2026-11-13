// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"net/http"

	session_module "gitea.dev/modules/session"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/openidConnect"
	"golang.org/x/oauth2"
)

func (source *Source) pkceSessionKey() string {
	return "oauth2_pkce_verifier:" + source.AuthSource.Name
}

// Callout redirects request/response pair to authenticate against the provider
func (source *Source) Callout(request *http.Request, response http.ResponseWriter) error {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(ProviderHeaderKey, source.AuthSource.Name)

	// don't use the default gothic begin handler to prevent issues when some error occurs
	// normally the gothic library will write some custom stuff to the response instead of our own nice error page
	// gothic.BeginAuthHandler(response, request)

	gothRWMutex.RLock()
	defer gothRWMutex.RUnlock()

	var pkceParams string
	if _, ok := goth.GetProviders()[source.AuthSource.Name].(*openidConnect.Provider); ok { // other goth providers don't send the code_verifier
		verifier := oauth2.GenerateVerifier()
		if err := session_module.GetContextSession(request).Set(source.pkceSessionKey(), verifier); err != nil {
			return err
		}
		pkceParams = "&code_challenge_method=S256&code_challenge=" + oauth2.S256ChallengeFromVerifier(verifier)
	}

	url, err := gothic.GetAuthURL(response, request)
	if err == nil {
		http.Redirect(response, request, url+pkceParams, http.StatusTemporaryRedirect)
	}
	return err
}

// Callback handles OAuth callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func (source *Source) Callback(request *http.Request, response http.ResponseWriter) (goth.User, error) {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(ProviderHeaderKey, source.AuthSource.Name)

	sess := session_module.GetContextSession(request)
	verifier, _ := sess.Get(source.pkceSessionKey()).(string)
	_ = sess.Delete(source.pkceSessionKey())
	request = request.Clone(request.Context())
	query := request.URL.Query()
	query.Set("code_verifier", verifier) // also when empty, to replace a callback-supplied one
	request.URL.RawQuery = query.Encode()

	gothRWMutex.RLock()
	defer gothRWMutex.RUnlock()

	user, err := gothic.CompleteUserAuth(response, request)
	if err != nil {
		return user, err
	}

	return user, nil
}

// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"net/http"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

// Callout redirects request/response pair to authenticate against the provider
func (source *Source) Callout(request *http.Request, response http.ResponseWriter) error {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(ProviderHeaderKey, source.AuthSource.Name)

	// don't use the default gothic begin handler to prevent issues when some error occurs
	// normally the gothic library will write some custom stuff to the response instead of our own nice error page
	// gothic.BeginAuthHandler(response, request)

	gothRWMutex.Lock()
	defer gothRWMutex.Unlock()

	updateProviderCallbackURL(goth.GetProviders()[source.AuthSource.Name], GetOAuth2CallbackURL(request.Context(), source.AuthSource.Name))

	url, err := gothic.GetAuthURL(response, request)
	if err == nil {
		http.Redirect(response, request, url, http.StatusTemporaryRedirect)
	}
	return err
}

// Callback handles OAuth callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func (source *Source) Callback(request *http.Request, response http.ResponseWriter) (goth.User, error) {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(ProviderHeaderKey, source.AuthSource.Name)

	gothRWMutex.Lock()
	defer gothRWMutex.Unlock()

	updateProviderCallbackURL(goth.GetProviders()[source.AuthSource.Name], GetOAuth2CallbackURL(request.Context(), source.AuthSource.Name))

	user, err := gothic.CompleteUserAuth(response, request)
	if err != nil {
		return user, err
	}

	return user, nil
}

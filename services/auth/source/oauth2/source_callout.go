// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"net/http"
	"slices"

	"gitea.dev/modules/httplib"
	"gitea.dev/modules/setting"

	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

func (source *Source) gothProviderName(req *http.Request) string {
	if appURL := httplib.GuessCurrentAppURL(req.Context()); slices.Contains(setting.AlternativeRootURLs, appURL) {
		return appURL + source.AuthSource.Name
	}
	return source.AuthSource.Name
}

// Callout redirects request/response pair to authenticate against the provider
func (source *Source) Callout(request *http.Request, response http.ResponseWriter) error {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(ProviderHeaderKey, source.gothProviderName(request))

	// don't use the default gothic begin handler to prevent issues when some error occurs
	// normally the gothic library will write some custom stuff to the response instead of our own nice error page
	// gothic.BeginAuthHandler(response, request)

	gothRWMutex.RLock()
	defer gothRWMutex.RUnlock()

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
	request.Header.Set(ProviderHeaderKey, source.gothProviderName(request))

	gothRWMutex.RLock()
	defer gothRWMutex.RUnlock()

	user, err := gothic.CompleteUserAuth(response, request)
	user.Provider = source.AuthSource.Name // alternative root providers are named by their registry key
	return user, err
}

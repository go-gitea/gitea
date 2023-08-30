// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import (
	"fmt"
	"net/http"

	"github.com/markbates/goth"
)

// Callout redirects request/response pair to authenticate against the provider
func (source *Source) Callout(request *http.Request, response http.ResponseWriter) error {
	samlRWMutex.RLock()
	defer samlRWMutex.RUnlock()
	if _, ok := providers[source.authSource.Name]; !ok {
		return fmt.Errorf("no provider for this saml")
	}

	authURL, err := providers[source.authSource.Name].samlSP.BuildAuthURL("")
	if err == nil {
		http.Redirect(response, request, authURL, http.StatusTemporaryRedirect)
	}
	return err
}

// Callback handles SAML callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func (source *Source) Callback(request *http.Request, response http.ResponseWriter) (goth.User, error) {
	samlRWMutex.RLock()
	defer samlRWMutex.RUnlock()

	user := goth.User{}
	samlResponse := request.FormValue("SAMLResponse")
	assertions, err := source.samlSP.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		return user, err
	}
	if warningInfo := assertions.WarningInfo; warningInfo != nil {
		return user, fmt.Errorf("SAML response contains warnings: %v", warningInfo)
	}

	return user, nil
}

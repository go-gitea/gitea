// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import (
	"fmt"
	"net/http"
	"strings"

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

	samlMap := make(map[string]string) // Global.
	for key, value := range assertions.Values {
		var (
			keyParsed   string
			valueParsed string
		)

		keyParsed = strings.ToLower(key[strings.LastIndex(key, "/")+1:]) // Uses the trailing slug as the key name.
		valueParsed = value.Values[0].Value
		samlMap[keyParsed] = valueParsed

	}

	user.UserID = assertions.NameID
	if user.UserID == "" {
		return user, fmt.Errorf("no nameID found in SAML response")
	}

	// email
	if _, ok := samlMap[source.EmailAssertionKey]; !ok {
		user.Email = samlMap[source.EmailAssertionKey]
	}
	// name
	if _, ok := samlMap[source.NameAssertionKey]; !ok {
		user.NickName = samlMap[source.NameAssertionKey]
	}
	// username
	if _, ok := samlMap[source.UsernameAssertionKey]; !ok {
		user.Name = samlMap[source.UsernameAssertionKey]
	}

	// TODO: utilize groups later on

	return user, nil
}

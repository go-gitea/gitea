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

	user := goth.User{
		Provider: source.authSource.Name,
	}
	samlResponse := request.FormValue("SAMLResponse")
	assertions, err := source.samlSP.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		return user, err
	}

	if assertions.WarningInfo.OneTimeUse {
		return user, fmt.Errorf("SAML response contains one time use warning")
	}

	if assertions.WarningInfo.ProxyRestriction != nil {
		return user, fmt.Errorf("SAML response contains proxy restriction warning: %v", assertions.WarningInfo.ProxyRestriction)
	}

	if assertions.WarningInfo.NotInAudience {
		return user, fmt.Errorf("SAML response contains audience warning")
	}

	if assertions.WarningInfo.InvalidTime {
		return user, fmt.Errorf("SAML response contains invalid time warning")
	}

	samlMap := make(map[string]string)
	for key, value := range assertions.Values {
		keyParsed := strings.ToLower(key[strings.LastIndex(key, "/")+1:]) // Uses the trailing slug as the key name.
		valueParsed := value.Values[0].Value
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

	// TODO: utilize groups once mapping is supported

	return user, nil
}

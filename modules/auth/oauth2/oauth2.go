// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"net/http"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/satori/go.uuid"
)

var (
	sessionUsersStoreKey = "gitea-oauth2-sessions"
	providerHeaderKey    = "gitea-oauth2-provider"
)

// Init initialize the setup of the OAuth2 library
func Init() {
	sessionDir := filepath.Join(setting.AppDataPath, "sessions", "oauth2")
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		log.Fatal(4, "Fail to create dir %s: %v", sessionDir, err)
	}

	gothic.Store = sessions.NewFilesystemStore(sessionDir, []byte(sessionUsersStoreKey))

	gothic.SetState = func(req *http.Request) string {
		return uuid.NewV4().String()
	}

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return req.Header.Get(providerHeaderKey), nil
	}

}

// Auth OAuth2 auth service
func Auth(provider string, request *http.Request, response http.ResponseWriter) error {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(providerHeaderKey, provider)

	// don't use the default gothic begin handler to prevent issues when some error occurs
	// normally the gothic library will write some custom stuff to the response instead of our own nice error page
	//gothic.BeginAuthHandler(response, request)

	url, err := gothic.GetAuthURL(response, request)
	if err == nil {
		http.Redirect(response, request, url, http.StatusTemporaryRedirect)
	}
	return err
}

// ProviderCallback handles OAuth callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func ProviderCallback(provider string, request *http.Request, response http.ResponseWriter) (goth.User, error) {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(providerHeaderKey, provider)

	user, err := gothic.CompleteUserAuth(response, request)
	if err != nil {
		return user, err
	}

	return user, nil
}

// RegisterProvider register a OAuth2 provider in goth lib
func RegisterProvider(providerName, providerType, clientID, clientSecret string) {
	provider := createProvider(providerName, providerType, clientID, clientSecret)

	if provider != nil {
		goth.UseProviders(provider)
	}
}

// RemoveProvider removes the given OAuth2 provider from the goth lib
func RemoveProvider(providerName string) {
	delete(goth.GetProviders(), providerName)
}

// used to create different types of goth providers
func createProvider(providerName, providerType, clientID, clientSecret string) goth.Provider {
	callbackURL := setting.AppURL + "user/oauth2/" + providerName + "/callback"

	var provider goth.Provider

	switch providerType {
	case "github":
		provider = github.New(clientID, clientSecret, callbackURL, "user:email")
	}

	// always set the name if provider is created so we can support multiple setups of 1 provider
	if provider != nil {
		provider.SetName(providerName)
	}

	return provider
}

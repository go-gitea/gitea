// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"math"
	"net/http"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/bitbucket"
	"github.com/markbates/goth/providers/dropbox"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/gitlab"
	"github.com/markbates/goth/providers/gplus"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/markbates/goth/providers/twitter"
	"github.com/satori/go.uuid"
)

var (
	sessionUsersStoreKey = "gitea-oauth2-sessions"
	providerHeaderKey    = "gitea-oauth2-provider"
)

// CustomURLMapping describes the urls values to use when customizing OAuth2 provider URLs
type CustomURLMapping struct {
	AuthURL    string
	TokenURL   string
	ProfileURL string
	EmailURL   string
}

// Init initialize the setup of the OAuth2 library
func Init() {
	sessionDir := filepath.Join(setting.AppDataPath, "sessions", "oauth2")
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		log.Fatal(4, "Fail to create dir %s: %v", sessionDir, err)
	}

	store := sessions.NewFilesystemStore(sessionDir, []byte(sessionUsersStoreKey))
	// according to the Goth lib:
	// set the maxLength of the cookies stored on the disk to a larger number to prevent issues with:
	// securecookie: the value is too long
	// when using OpenID Connect , since this can contain a large amount of extra information in the id_token

	// Note, when using the FilesystemStore only the session.ID is written to a browser cookie, so this is explicit for the storage on disk
	store.MaxLength(math.MaxInt16)
	gothic.Store = store

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
func RegisterProvider(providerName, providerType, clientID, clientSecret, openIDConnectAutoDiscoveryURL string, customURLMapping *CustomURLMapping) error {
	provider, err := createProvider(providerName, providerType, clientID, clientSecret, openIDConnectAutoDiscoveryURL, customURLMapping)

	if err == nil && provider != nil {
		goth.UseProviders(provider)
	}

	return err
}

// RemoveProvider removes the given OAuth2 provider from the goth lib
func RemoveProvider(providerName string) {
	delete(goth.GetProviders(), providerName)
}

// used to create different types of goth providers
func createProvider(providerName, providerType, clientID, clientSecret, openIDConnectAutoDiscoveryURL string, customURLMapping *CustomURLMapping) (goth.Provider, error) {
	callbackURL := setting.AppURL + "user/oauth2/" + providerName + "/callback"

	var provider goth.Provider
	var err error

	switch providerType {
	case "bitbucket":
		provider = bitbucket.New(clientID, clientSecret, callbackURL, "account")
	case "dropbox":
		provider = dropbox.New(clientID, clientSecret, callbackURL)
	case "facebook":
		provider = facebook.New(clientID, clientSecret, callbackURL, "email")
	case "github":
		authURL := github.AuthURL
		tokenURL := github.TokenURL
		profileURL := github.ProfileURL
		emailURL := github.EmailURL
		if customURLMapping != nil {
			if len(customURLMapping.AuthURL) > 0 {
				authURL = customURLMapping.AuthURL
			}
			if len(customURLMapping.TokenURL) > 0 {
				tokenURL = customURLMapping.TokenURL
			}
			if len(customURLMapping.ProfileURL) > 0 {
				profileURL = customURLMapping.ProfileURL
			}
			if len(customURLMapping.EmailURL) > 0 {
				emailURL = customURLMapping.EmailURL
			}
		}
		provider = github.NewCustomisedURL(clientID, clientSecret, callbackURL, authURL, tokenURL, profileURL, emailURL)
	case "gitlab":
		authURL := gitlab.AuthURL
		tokenURL := gitlab.TokenURL
		profileURL := gitlab.ProfileURL
		if customURLMapping != nil {
			if len(customURLMapping.AuthURL) > 0 {
				authURL = customURLMapping.AuthURL
			}
			if len(customURLMapping.TokenURL) > 0 {
				tokenURL = customURLMapping.TokenURL
			}
			if len(customURLMapping.ProfileURL) > 0 {
				profileURL = customURLMapping.ProfileURL
			}
		}
		provider = gitlab.NewCustomisedURL(clientID, clientSecret, callbackURL, authURL, tokenURL, profileURL)
	case "gplus":
		provider = gplus.New(clientID, clientSecret, callbackURL, "email")
	case "openidConnect":
		if provider, err = openidConnect.New(clientID, clientSecret, callbackURL, openIDConnectAutoDiscoveryURL); err != nil {
			log.Warn("Failed to create OpenID Connect Provider with name '%s' with url '%s': %v", providerName, openIDConnectAutoDiscoveryURL, err)
		}
	case "twitter":
		provider = twitter.NewAuthenticate(clientID, clientSecret, callbackURL)
	}

	// always set the name if provider is created so we can support multiple setups of 1 provider
	if err == nil && provider != nil {
		provider.SetName(providerName)
	}

	return provider, err
}

// GetDefaultTokenURL return the default token url for the given provider
func GetDefaultTokenURL(provider string) string {
	switch provider {
	case "github":
		return github.TokenURL
	case "gitlab":
		return gitlab.TokenURL
	}
	return ""
}

// GetDefaultAuthURL return the default authorize url for the given provider
func GetDefaultAuthURL(provider string) string {
	switch provider {
	case "github":
		return github.AuthURL
	case "gitlab":
		return gitlab.AuthURL
	}
	return ""
}

// GetDefaultProfileURL return the default profile url for the given provider
func GetDefaultProfileURL(provider string) string {
	switch provider {
	case "github":
		return github.ProfileURL
	case "gitlab":
		return gitlab.ProfileURL
	}
	return ""
}

// GetDefaultEmailURL return the default email url for the given provider
func GetDefaultEmailURL(provider string) string {
	switch provider {
	case "github":
		return github.EmailURL
	}
	return ""
}

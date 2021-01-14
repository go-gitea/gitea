// Package nextcloud implements the OAuth2 protocol for authenticating users through nextcloud.
// This package can be used as a reference implementation of an OAuth2 provider for Goth.
package nextcloud

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"fmt"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

// These vars define the Authentication, Token, and Profile URLS for Nextcloud.
// You have to set these values to something useful, because nextcloud is always
// hosted somewhere.
//
var (
	AuthURL    = "https://<own-server>/apps/oauth2/authorize"
	TokenURL   = "https://<own-server>/apps/oauth2/api/v1/token"
	ProfileURL = "https://<own-server>/ocs/v2.php/cloud/user?format=json"
)

// Provider is the implementation of `goth.Provider` for accessing Nextcloud.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	config       *oauth2.Config
	providerName string
	authURL      string
	tokenURL     string
	profileURL   string
}

// New is only here to fulfill the interface requirements and does not work properly without
// setting your own Nextcloud connect parameters, more precisely AuthURL, TokenURL and ProfileURL.
// Please use NewCustomisedDNS with the beginning of your URL or NewCustomiseURL.
func New(clientKey, secret, callbackURL string, scopes ...string) *Provider {
	return NewCustomisedURL(clientKey, secret, callbackURL, AuthURL, TokenURL, ProfileURL, scopes...)
}

// NewCustomisedURL create a working connection to your Nextcloud server given by the values
// authURL, tokenURL and profileURL.
// If you want to use a simpler method, please have a look at NewCustomisedDNS, which gets only
// on parameter instead of three.
func NewCustomisedURL(clientKey, secret, callbackURL, authURL, tokenURL, profileURL string, scopes ...string) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "nextcloud",
		profileURL:   profileURL,
	}
	p.config = newConfig(p, authURL, tokenURL, scopes)
	return p
}

// NewCustomisedDNS is the simplest method to create a provider based only on your key/secret
// and the beginning of the URL to your server, e.g. https://my.server.name/
func NewCustomisedDNS(clientKey, secret, callbackURL, nextcloudURL string, scopes ...string) *Provider {
	return NewCustomisedURL(
		clientKey,
		secret,
		callbackURL,
		nextcloudURL+"/apps/oauth2/authorize",
		nextcloudURL+"/apps/oauth2/api/v1/token",
		nextcloudURL+"/ocs/v2.php/cloud/user?format=json",
		scopes...,
	)
}

// Name is the name used to retrieve this provider later.
func (p *Provider) Name() string {
	return p.providerName
}

// SetName is to update the name of the provider (needed in case of multiple providers of 1 type)
func (p *Provider) SetName(name string) {
	p.providerName = name
}

func (p *Provider) Client() *http.Client {
	return goth.HTTPClientWithFallBack(p.HTTPClient)
}

// Debug is a no-op for the nextcloud package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks Nextcloud for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	return &Session{
		AuthURL: p.config.AuthCodeURL(state),
	}, nil
}

// FetchUser will go to Nextcloud and access basic information about the user.
func (p *Provider) FetchUser(session goth.Session) (goth.User, error) {
	sess := session.(*Session)
	user := goth.User{
		AccessToken:  sess.AccessToken,
		Provider:     p.Name(),
		RefreshToken: sess.RefreshToken,
		ExpiresAt:    sess.ExpiresAt,
	}

	if user.AccessToken == "" {
		// data is not yet retrieved since accessToken is still empty
		return user, fmt.Errorf("%s cannot get user information without accessToken", p.providerName)
	}

	req, err := http.NewRequest("GET", p.profileURL, nil)
	if err != nil {
		return user, err
	}
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	response, err := p.Client().Do(req)

	if err != nil {
		if response != nil {
			response.Body.Close()
		}
		return user, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return user, fmt.Errorf("%s responded with a %d trying to fetch user information", p.providerName, response.StatusCode)
	}

	bits, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return user, err
	}

	err = json.NewDecoder(bytes.NewReader(bits)).Decode(&user.RawData)
	if err != nil {
		return user, err
	}

	err = userFromReader(bytes.NewReader(bits), &user)

	return user, err
}

func newConfig(provider *Provider, authURL, tokenURL string, scopes []string) *oauth2.Config {
	c := &oauth2.Config{
		ClientID:     provider.ClientKey,
		ClientSecret: provider.Secret,
		RedirectURL:  provider.CallbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		Scopes: []string{},
	}

	if len(scopes) > 0 {
		for _, scope := range scopes {
			c.Scopes = append(c.Scopes, scope)
		}
	}
	return c
}

func userFromReader(r io.Reader, user *goth.User) error {
	u := struct {
		Ocs struct {
			Data struct {
				EMail       string `json:"email"`
				DisplayName string `json:"display-name"`
				ID          string `json:"id"`
				Address     string `json:"address"`
			}
		} `json:"ocs"`
	}{}
	err := json.NewDecoder(r).Decode(&u)
	if err != nil {
		return err
	}
	user.Email = u.Ocs.Data.EMail
	user.Name = u.Ocs.Data.DisplayName
	user.UserID = u.Ocs.Data.ID
	user.Location = u.Ocs.Data.Address
	return nil
}

//RefreshTokenAvailable refresh token is provided by auth provider or not
func (p *Provider) RefreshTokenAvailable() bool {
	return true
}

//RefreshToken get new access token based on the refresh token
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{RefreshToken: refreshToken}
	ts := p.config.TokenSource(goth.ContextForClient(p.Client()), token)
	newToken, err := ts.Token()
	if err != nil {
		return nil, err
	}
	return newToken, err
}

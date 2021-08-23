// Package azuread implements the OAuth2 protocol for authenticating users through AzureAD.
// This package can be used as a reference implementation of an OAuth2 provider for Goth.
// To use microsoft personal account use microsoftonline provider
package azuread

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

const (
	authURL          string = "https://login.microsoftonline.com/common/oauth2/authorize"
	tokenURL         string = "https://login.microsoftonline.com/common/oauth2/token"
	endpointProfile  string = "https://graph.windows.net/me?api-version=1.6"
	graphAPIResource string = "https://graph.windows.net/"
)

// New creates a new AzureAD provider, and sets up important connection details.
// You should always call `AzureAD.New` to get a new Provider. Never try to create
// one manually.
func New(clientKey, secret, callbackURL string, resources []string, scopes ...string) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "azuread",
	}

	p.resources = make([]string, 0, 1+len(resources))
	p.resources = append(p.resources, graphAPIResource)
	p.resources = append(p.resources, resources...)

	p.config = newConfig(p, scopes)
	return p
}

// Provider is the implementation of `goth.Provider` for accessing AzureAD.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	config       *oauth2.Config
	providerName string
	resources    []string
}

// Name is the name used to retrieve this provider later.
func (p *Provider) Name() string {
	return p.providerName
}

// SetName is to update the name of the provider (needed in case of multiple providers of 1 type)
func (p *Provider) SetName(name string) {
	p.providerName = name
}

// Client is HTTP client to be used in all fetch operations.
func (p *Provider) Client() *http.Client {
	return goth.HTTPClientWithFallBack(p.HTTPClient)
}

// Debug is a no-op for the package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks AzureAD for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	authURL := p.config.AuthCodeURL(state)

	// Azure ad requires at least one resource
	authURL += "&resource=" + url.QueryEscape(strings.Join(p.resources, " "))

	return &Session{
		AuthURL: authURL,
	}, nil
}

// FetchUser will go to AzureAD and access basic information about the user.
func (p *Provider) FetchUser(session goth.Session) (goth.User, error) {
	msSession := session.(*Session)
	user := goth.User{
		AccessToken: msSession.AccessToken,
		Provider:    p.Name(),
		ExpiresAt:   msSession.ExpiresAt,
	}

	if user.AccessToken == "" {
		return user, fmt.Errorf("%s cannot get user information without accessToken", p.providerName)
	}

	req, err := http.NewRequest("GET", endpointProfile, nil)
	if err != nil {
		return user, err
	}

	req.Header.Set(authorizationHeader(msSession))

	response, err := p.Client().Do(req)
	if err != nil {
		return user, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return user, fmt.Errorf("%s responded with a %d trying to fetch user information", p.providerName, response.StatusCode)
	}

	err = userFromReader(response.Body, &user)
	return user, err
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

func newConfig(provider *Provider, scopes []string) *oauth2.Config {
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
	} else {
		c.Scopes = append(c.Scopes, "user_impersonation")
	}

	return c
}

func userFromReader(r io.Reader, user *goth.User) error {
	u := struct {
		Name              string `json:"name"`
		Email             string `json:"mail"`
		FirstName         string `json:"givenName"`
		LastName          string `json:"surname"`
		NickName          string `json:"mailNickname"`
		UserPrincipalName string `json:"userPrincipalName"`
		Location          string `json:"usageLocation"`
	}{}

	err := json.NewDecoder(r).Decode(&u)
	if err != nil {
		return err
	}

	user.Email = u.Email
	user.Name = u.Name
	user.FirstName = u.FirstName
	user.LastName = u.LastName
	user.NickName = u.Name
	user.Location = u.Location
	user.UserID = u.UserPrincipalName //AzureAD doesn't provide separate user_id

	return nil
}

func authorizationHeader(session *Session) (string, string) {
	return "Authorization", fmt.Sprintf("Bearer %s", session.AccessToken)
}

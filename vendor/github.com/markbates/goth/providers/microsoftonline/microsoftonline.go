// Package microsoftonline implements the OAuth2 protocol for authenticating users through microsoftonline.
// This package can be used as a reference implementation of an OAuth2 provider for Goth.
// To use this package, your application need to be registered in [Application Registration Portal](https://apps.dev.microsoft.com/)
package microsoftonline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/markbates/going/defaults"
	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

const (
	authURL         string = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	tokenURL        string = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	endpointProfile string = "https://graph.microsoft.com/v1.0/me"
)

var defaultScopes = []string{"openid", "offline_access", "user.read"}

// New creates a new microsoftonline provider, and sets up important connection details.
// You should always call `microsoftonline.New` to get a new Provider. Never try to create
// one manually.
func New(clientKey, secret, callbackURL string, scopes ...string) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "microsoftonline",
	}

	p.config = newConfig(p, scopes)
	return p
}

// Provider is the implementation of `goth.Provider` for accessing microsoftonline.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	config       *oauth2.Config
	providerName string
	tenant       string
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

// Debug is a no-op for the facebook package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks MicrosoftOnline for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	authURL := p.config.AuthCodeURL(state)
	return &Session{
		AuthURL: authURL,
	}, nil
}

// FetchUser will go to MicrosoftOnline and access basic information about the user.
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

	user.AccessToken = msSession.AccessToken

	err = userFromReader(response.Body, &user)
	return user, err
}

// RefreshTokenAvailable refresh token is provided by auth provider or not
// not available for microsoft online as session size hit the limit of max cookie size
func (p *Provider) RefreshTokenAvailable() bool {
	return false
}

//RefreshToken get new access token based on the refresh token
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("No refresh token provided")
	}

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

	c.Scopes = append(c.Scopes, scopes...)
	if len(scopes) == 0 {
		c.Scopes = append(c.Scopes, defaultScopes...)
	}

	return c
}

func userFromReader(r io.Reader, user *goth.User) error {
	buf := &bytes.Buffer{}
	tee := io.TeeReader(r, buf)

	u := struct {
		ID                string `json:"id"`
		Name              string `json:"displayName"`
		Email             string `json:"mail"`
		FirstName         string `json:"givenName"`
		LastName          string `json:"surname"`
		UserPrincipalName string `json:"userPrincipalName"`
	}{}

	if err := json.NewDecoder(tee).Decode(&u); err != nil {
		return err
	}

	raw := map[string]interface{}{}
	if err := json.NewDecoder(buf).Decode(&raw); err != nil {
		return err
	}

	user.UserID = u.ID
	user.Email = defaults.String(u.Email, u.UserPrincipalName)
	user.Name = u.Name
	user.NickName = u.Name
	user.FirstName = u.FirstName
	user.LastName = u.LastName
	user.RawData = raw

	return nil
}

func authorizationHeader(session *Session) (string, string) {
	return "Authorization", fmt.Sprintf("Bearer %s", session.AccessToken)
}

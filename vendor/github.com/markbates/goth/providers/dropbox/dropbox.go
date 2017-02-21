// Package dropbox implements the OAuth2 protocol for authenticating users through Dropbox.
package dropbox

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
	"fmt"
)

const (
	authURL    = "https://www.dropbox.com/1/oauth2/authorize"
	tokenURL   = "https://api.dropbox.com/1/oauth2/token"
	accountURL = "https://api.dropbox.com/1/account/info"
)

// Provider is the implementation of `goth.Provider` for accessing Dropbox.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	config       *oauth2.Config
	providerName string
}

// Session stores data during the auth process with Dropbox.
type Session struct {
	AuthURL string
	Token   string
}

// New creates a new Dropbox provider and sets up important connection details.
// You should always call `dropbox.New` to get a new provider.  Never try to
// create one manually.
func New(clientKey, secret, callbackURL string, scopes ...string) *Provider {
	p := &Provider{
		ClientKey:           clientKey,
		Secret:              secret,
		CallbackURL:         callbackURL,
		providerName:        "dropbox",
	}
	p.config = newConfig(p, scopes)
	return p
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

// Debug is a no-op for the dropbox package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks Dropbox for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	return &Session{
		AuthURL: p.config.AuthCodeURL(state),
	}, nil
}

// FetchUser will go to Dropbox and access basic information about the user.
func (p *Provider) FetchUser(session goth.Session) (goth.User, error) {
	s := session.(*Session)
	user := goth.User{
		AccessToken: s.Token,
		Provider:    p.Name(),
	}

	if user.AccessToken == "" {
		// data is not yet retrieved since accessToken is still empty
		return user, fmt.Errorf("%s cannot get user information without accessToken", p.providerName)
	}

	req, err := http.NewRequest("GET", accountURL, nil)
	if err != nil {
		return user, err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := p.Client().Do(req)
	if err != nil {
		return user, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return user, fmt.Errorf("%s responded with a %d trying to fetch user information", p.providerName, resp.StatusCode)
	}

	err = userFromReader(resp.Body, &user)
	return user, err
}

// UnmarshalSession wil unmarshal a JSON string into a session.
func (p *Provider) UnmarshalSession(data string) (goth.Session, error) {
	s := &Session{}
	err := json.NewDecoder(strings.NewReader(data)).Decode(s)
	return s, err
}

// GetAuthURL gets the URL set by calling the `BeginAuth` function on the Dropbox provider.
func (s *Session) GetAuthURL() (string, error) {
	if s.AuthURL == "" {
		return "", errors.New("dropbox: missing AuthURL")
	}
	return s.AuthURL, nil
}

// Authorize the session with Dropbox and return the access token to be stored for future use.
func (s *Session) Authorize(provider goth.Provider, params goth.Params) (string, error) {
	p := provider.(*Provider)
	token, err := p.config.Exchange(goth.ContextForClient(p.Client()), params.Get("code"))
	if err != nil {
		return "", err
	}

	if !token.Valid() {
		return "", errors.New("Invalid token received from provider")
	}

	s.Token = token.AccessToken
	return token.AccessToken, nil
}

// Marshal the session into a string
func (s *Session) Marshal() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (s Session) String() string {
	return s.Marshal()
}

func newConfig(p *Provider, scopes []string) *oauth2.Config {
	c := &oauth2.Config{
		ClientID:     p.ClientKey,
		ClientSecret: p.Secret,
		RedirectURL:  p.CallbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}
	return c
}

func userFromReader(r io.Reader, user *goth.User) error {
	u := struct {
		Name string `json:"display_name"`
		NameDetails struct {
			NickName string `json:"familiar_name"`
		} `json:"name_details"`
		Location string `json:"country"`
		Email    string `json:"email"`
	}{}
	err := json.NewDecoder(r).Decode(&u)
	if err != nil {
		return err
	}
	user.Email = u.Email
	user.Name = u.Name
	user.NickName = u.NameDetails.NickName
	user.UserID = u.Email // Dropbox doesn't provide a separate user ID
	user.Location = u.Location
	return nil
}

//RefreshToken refresh token is not provided by dropbox
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	return nil, errors.New("Refresh token is not provided by dropbox")
}

//RefreshTokenAvailable refresh token is not provided by dropbox
func (p *Provider) RefreshTokenAvailable() bool {
	return false
}

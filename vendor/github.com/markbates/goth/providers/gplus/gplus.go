// Package gplus implements the OAuth2 protocol for authenticating users through Google+.
// This package can be used as a reference implementation of an OAuth2 provider for Goth.
package gplus

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
	"fmt"
)

const (
	authURL         string = "https://accounts.google.com/o/oauth2/auth?access_type=offline"
	tokenURL        string = "https://accounts.google.com/o/oauth2/token"
	endpointProfile string = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// New creates a new Google+ provider, and sets up important connection details.
// You should always call `gplus.New` to get a new Provider. Never try to create
// one manually.
func New(clientKey, secret, callbackURL string, scopes ...string) *Provider {
	p := &Provider{
		ClientKey:           clientKey,
		Secret:              secret,
		CallbackURL:         callbackURL,
		providerName:        "gplus",
	}
	p.config = newConfig(p, scopes)
	return p
}

// Provider is the implementation of `goth.Provider` for accessing Google+.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	config       *oauth2.Config
	prompt       oauth2.AuthCodeOption
	providerName string
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

// Debug is a no-op for the gplus package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks Google+ for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	var opts []oauth2.AuthCodeOption
	if p.prompt != nil {
		opts = append(opts, p.prompt)
	}
	url := p.config.AuthCodeURL(state, opts...)
	session := &Session{
		AuthURL: url,
	}
	return session, nil
}

// FetchUser will go to Google+ and access basic information about the user.
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

	response, err := p.Client().Get(endpointProfile + "?access_token=" + url.QueryEscape(sess.AccessToken))
	if err != nil {
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

func userFromReader(reader io.Reader, user *goth.User) error {
	u := struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		FirstName string `json:"given_name"`
		LastName  string `json:"family_name"`
		Link      string `json:"link"`
		Picture   string `json:"picture"`
	}{}

	err := json.NewDecoder(reader).Decode(&u)
	if err != nil {
		return err
	}

	user.Name = u.Name
	user.FirstName = u.FirstName
	user.LastName = u.LastName
	user.NickName = u.Name
	user.Email = u.Email
	//user.Description = u.Bio
	user.AvatarURL = u.Picture
	user.UserID = u.ID
	//user.Location = u.Location.Name

	return err
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
		c.Scopes = []string{"profile", "email", "openid"}
	}
	return c
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

// SetPrompt sets the prompt values for the GPlus OAuth call. Use this to
// force users to choose and account every time by passing "select_account",
// for example.
// See https://developers.google.com/identity/protocols/OpenIDConnect#authenticationuriparameters
func (p *Provider) SetPrompt(prompt ...string) {
	if len(prompt) == 0 {
		return
	}
	p.prompt = oauth2.SetAuthURLParam("prompt", strings.Join(prompt, " "))
}

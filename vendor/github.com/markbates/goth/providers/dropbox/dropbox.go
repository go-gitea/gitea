// Package dropbox implements the OAuth2 protocol for authenticating users through Dropbox.
package dropbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"fmt"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

const (
	authURL    = "https://www.dropbox.com/oauth2/authorize"
	tokenURL   = "https://api.dropbox.com/oauth2/token"
	accountURL = "https://api.dropbox.com/2/users/get_current_account"
)

// Provider is the implementation of `goth.Provider` for accessing Dropbox.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	AccountURL   string
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
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		AccountURL:   accountURL,
		providerName: "dropbox",
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

	req, err := http.NewRequest("POST", p.AccountURL, nil)
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

	bits, err := ioutil.ReadAll(resp.Body)
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
		AccountID string `json:"account_id"`
		Name      struct {
			GivenName   string `json:"given_name"`
			Surname     string `json:"surname"`
			DisplayName string `json:"display_name"`
		} `json:"name"`
		Country         string `json:"country"`
		Email           string `json:"email"`
		ProfilePhotoURL string `json:"profile_photo_url"`
	}{}
	err := json.NewDecoder(r).Decode(&u)
	if err != nil {
		return err
	}
	user.UserID = u.AccountID // The user's unique Dropbox ID.
	user.FirstName = u.Name.GivenName
	user.LastName = u.Name.Surname
	user.Name = strings.TrimSpace(fmt.Sprintf("%s %s", u.Name.GivenName, u.Name.Surname))
	user.Description = u.Name.DisplayName // Full name plus parenthetical team naem
	user.Email = u.Email
	user.NickName = u.Email // Email is the dropbox username
	user.Location = u.Country
	user.AvatarURL = u.ProfilePhotoURL // May be blank
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

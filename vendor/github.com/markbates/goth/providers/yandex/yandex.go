// package yandex implements the OAuth2 protocol for authenticating users through Yandex.
// This package can be used as a reference implementation of an OAuth2 provider for Goth.
package yandex

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

const (
	authEndpoint    string = "https://oauth.yandex.ru/authorize"
	tokenEndpoint   string = "https://oauth.yandex.com/token"
	profileEndpoint string = "https://login.yandex.ru/info"
	avatarURL       string = "https://avatars.yandex.net/get-yapic"
	avatarSize      string = "islands-200"
)

// Provider is the implementation of `goth.Provider` for accessing Yandex.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	config       *oauth2.Config
	providerName string
}

// New creates a new Yandex provider and sets up important connection details.
// You should always call `yandex.New` to get a new provider.  Never try to
// create one manually.
func New(clientKey, secret, callbackURL string, scopes ...string) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "yandex",
	}
	p.config = newConfig(p, scopes)
	return p
}

func (p *Provider) Client() *http.Client {
	return goth.HTTPClientWithFallBack(p.HTTPClient)
}

// Name is the name used to retrieve this provider later.
func (p *Provider) Name() string {
	return p.providerName
}

// SetName is to update the name of the provider (needed in case of multiple providers of 1 type)
func (p *Provider) SetName(name string) {
	p.providerName = name
}

// Debug is a no-op for the yandex package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks Yandex for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	return &Session{
		AuthURL: p.config.AuthCodeURL(state),
	}, nil
}

// FetchUser will go to Yandex and access basic information about the user.
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

	req, err := http.NewRequest("GET", profileEndpoint, nil)
	if err != nil {
		return user, err
	}
	req.Header.Set("Authorization", "OAuth "+sess.AccessToken)
	resp, err := p.Client().Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
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

func newConfig(provider *Provider, scopes []string) *oauth2.Config {
	c := &oauth2.Config{
		ClientID:     provider.ClientKey,
		ClientSecret: provider.Secret,
		RedirectURL:  provider.CallbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authEndpoint,
			TokenURL: tokenEndpoint,
		},
		Scopes: []string{},
	}
	if len(scopes) > 0 {
		for _, scope := range scopes {
			c.Scopes = append(c.Scopes, scope)
		}
	} else {
		c.Scopes = append(c.Scopes, "login:email login:info login:avatar")
	}
	return c
}

func userFromReader(r io.Reader, user *goth.User) error {
	u := struct {
		UserID        string `json:"id"`
		Email         string `json:"default_email"`
		Login         string `json:"login"`
		Name          string `json:"real_name"`
		FirstName     string `json:"first_name"`
		LastName      string `json:"last_name"`
		AvatarID      string `json:"default_avatar_id"`
		IsAvatarEmpty bool   `json:"is_avatar_empty"`
	}{}

	err := json.NewDecoder(r).Decode(&u)
	if err != nil {
		return err
	}
	user.UserID = u.UserID
	user.Email = u.Email
	user.NickName = u.Login
	user.Name = u.Name
	user.FirstName = u.FirstName
	user.LastName = u.LastName
	if u.AvatarID != `` {
		user.AvatarURL = fmt.Sprintf("%s/%s/%s", avatarURL, u.AvatarID, avatarSize)
	}
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

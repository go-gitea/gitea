// Package twitter implements the OAuth protocol for authenticating users through Twitter.
// This package can be used as a reference implementation of an OAuth provider for Goth.
package twitter

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"fmt"

	"github.com/markbates/goth"
	"github.com/mrjones/oauth"
	"golang.org/x/oauth2"
)

var (
	requestURL      = "https://api.twitter.com/oauth/request_token"
	authorizeURL    = "https://api.twitter.com/oauth/authorize"
	authenticateURL = "https://api.twitter.com/oauth/authenticate"
	tokenURL        = "https://api.twitter.com/oauth/access_token"
	endpointProfile = "https://api.twitter.com/1.1/account/verify_credentials.json"
)

// New creates a new Twitter provider, and sets up important connection details.
// You should always call `twitter.New` to get a new Provider. Never try to create
// one manually.
//
// If you'd like to use authenticate instead of authorize, use NewAuthenticate instead.
func New(clientKey, secret, callbackURL string) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "twitter",
	}
	p.consumer = newConsumer(p, authorizeURL)
	return p
}

// NewAuthenticate is the almost same as New.
// NewAuthenticate uses the authenticate URL instead of the authorize URL.
func NewAuthenticate(clientKey, secret, callbackURL string) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "twitter",
	}
	p.consumer = newConsumer(p, authenticateURL)
	return p
}

// Provider is the implementation of `goth.Provider` for accessing Twitter.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	debug        bool
	consumer     *oauth.Consumer
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

// Debug sets the logging of the OAuth client to verbose.
func (p *Provider) Debug(debug bool) {
	p.debug = debug
}

// BeginAuth asks Twitter for an authentication end-point and a request token for a session.
// Twitter does not support the "state" variable.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	requestToken, url, err := p.consumer.GetRequestTokenAndUrl(p.CallbackURL)
	session := &Session{
		AuthURL:      url,
		RequestToken: requestToken,
	}
	return session, err
}

// FetchUser will go to Twitter and access basic information about the user.
func (p *Provider) FetchUser(session goth.Session) (goth.User, error) {
	sess := session.(*Session)
	user := goth.User{
		Provider: p.Name(),
	}

	if sess.AccessToken == nil {
		// data is not yet retrieved since accessToken is still empty
		return user, fmt.Errorf("%s cannot get user information without accessToken", p.providerName)
	}

	response, err := p.consumer.Get(
		endpointProfile,
		map[string]string{"include_entities": "false", "skip_status": "true", "include_email": "true"},
		sess.AccessToken)
	if err != nil {
		return user, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return user, fmt.Errorf("%s responded with a %d trying to fetch user information", p.providerName, response.StatusCode)
	}

	bits, err := ioutil.ReadAll(response.Body)
	err = json.NewDecoder(bytes.NewReader(bits)).Decode(&user.RawData)
	if err != nil {
		return user, err
	}

	user.Name = user.RawData["name"].(string)
	user.NickName = user.RawData["screen_name"].(string)
	if user.RawData["email"] != nil {
		user.Email = user.RawData["email"].(string)
	}
	user.Description = user.RawData["description"].(string)
	user.AvatarURL = user.RawData["profile_image_url"].(string)
	user.UserID = user.RawData["id_str"].(string)
	user.Location = user.RawData["location"].(string)
	user.AccessToken = sess.AccessToken.Token
	user.AccessTokenSecret = sess.AccessToken.Secret
	return user, err
}

func newConsumer(provider *Provider, authURL string) *oauth.Consumer {
	c := oauth.NewConsumer(
		provider.ClientKey,
		provider.Secret,
		oauth.ServiceProvider{
			RequestTokenUrl:   requestURL,
			AuthorizeTokenUrl: authURL,
			AccessTokenUrl:    tokenURL,
		})

	c.Debug(provider.debug)
	return c
}

//RefreshToken refresh token is not provided by twitter
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	return nil, errors.New("Refresh token is not provided by twitter")
}

//RefreshTokenAvailable refresh token is not provided by twitter
func (p *Provider) RefreshTokenAvailable() bool {
	return false
}

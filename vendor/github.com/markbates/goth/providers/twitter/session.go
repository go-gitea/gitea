package twitter

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/markbates/goth"
	"github.com/mrjones/oauth"
)

// Session stores data during the auth process with Twitter.
type Session struct {
	AuthURL      string
	AccessToken  *oauth.AccessToken
	RequestToken *oauth.RequestToken
}

// GetAuthURL will return the URL set by calling the `BeginAuth` function on the Twitter provider.
func (s Session) GetAuthURL() (string, error) {
	if s.AuthURL == "" {
		return "", errors.New(goth.NoAuthUrlErrorMessage)
	}
	return s.AuthURL, nil
}

// Authorize the session with Twitter and return the access token to be stored for future use.
func (s *Session) Authorize(provider goth.Provider, params goth.Params) (string, error) {
	p := provider.(*Provider)
	accessToken, err := p.consumer.AuthorizeToken(s.RequestToken, params.Get("oauth_verifier"))
	if err != nil {
		return "", err
	}

	s.AccessToken = accessToken
	return accessToken.Token, err
}

// Marshal the session into a string
func (s Session) Marshal() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (s Session) String() string {
	return s.Marshal()
}

// UnmarshalSession will unmarshal a JSON string into a session.
func (p *Provider) UnmarshalSession(data string) (goth.Session, error) {
	sess := &Session{}
	err := json.NewDecoder(strings.NewReader(data)).Decode(sess)
	return sess, err
}

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
// Package blenderid implements the OAuth2 protocol for authenticating users through Blender ID
// This package can be used as a reference implementation of an OAuth2 provider for Goth.
package blenderid

// Allow "encoding/json" import.
import (
	"bytes"
	"encoding/json" //nolint:depguard
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

// These vars define the default Authentication, Token, and Profile URLS for Blender ID
//
// Examples:
//
//	blenderid.AuthURL = "https://id.blender.org/oauth/authorize
//	blenderid.TokenURL = "https://id.blender.org/oauth/token
//	blenderid.ProfileURL = "https://id.blender.org/api/me
var (
	AuthURL    = "https://id.blender.org/oauth/authorize"
	TokenURL   = "https://id.blender.org/oauth/token"
	ProfileURL = "https://id.blender.org/api/me"
)

// Provider is the implementation of `goth.Provider` for accessing Blender ID
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	config       *oauth2.Config
	providerName string
	profileURL   string
}

// New creates a new Blender ID provider and sets up important connection details.
// You should always call `blenderid.New` to get a new provider.  Never try to
// create one manually.
func New(clientKey, secret, callbackURL string, scopes ...string) *Provider {
	return NewCustomisedURL(clientKey, secret, callbackURL, AuthURL, TokenURL, ProfileURL, scopes...)
}

// NewCustomisedURL is similar to New(...) but can be used to set custom URLs to connect to
func NewCustomisedURL(clientKey, secret, callbackURL, authURL, tokenURL, profileURL string, scopes ...string) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "blenderid",
		profileURL:   profileURL,
	}
	p.config = newConfig(p, authURL, tokenURL, scopes)
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

// Debug is a no-op for the blenderid package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks Blender ID for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	return &Session{
		AuthURL: p.config.AuthCodeURL(state),
	}, nil
}

// FetchUser will go to Blender ID and access basic information about the user.
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

	req.Header.Add("Authorization", "Bearer "+sess.AccessToken)
	response, err := p.Client().Do(req)
	if err != nil {
		return user, err
	}
	if response.StatusCode != http.StatusOK {
		return user, fmt.Errorf("Blender ID responded with a %d trying to fetch user information", response.StatusCode)
	}

	bits, err := io.ReadAll(response.Body)
	if err != nil {
		return user, err
	}

	err = json.NewDecoder(bytes.NewReader(bits)).Decode(&user.RawData)
	if err != nil {
		return user, err
	}

	err = userFromReader(bytes.NewReader(bits), &user)
	if err != nil {
		return user, err
	}

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
		c.Scopes = append(c.Scopes, scopes...)
	}
	return c
}

func userFromReader(r io.Reader, user *goth.User) error {
	u := struct {
		Name     string `json:"full_name"`
		Email    string `json:"email"`
		NickName string `json:"nickname"`
		ID       int    `json:"id"`
	}{}
	err := json.NewDecoder(r).Decode(&u)
	if err != nil {
		return err
	}
	user.Email = u.Email
	user.Name = u.Name
	user.NickName = gitealizeUsername(u.NickName)
	user.UserID = strconv.Itoa(u.ID)
	user.AvatarURL = fmt.Sprintf("https://id.blender.org/api/user/%s/avatar", user.UserID)
	return nil
}

// RefreshTokenAvailable refresh token is not provided by Blender ID
func (p *Provider) RefreshTokenAvailable() bool {
	return true
}

// RefreshToken refresh token is not provided by Blender ID
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	return nil, errors.New("Refresh token is not provided by Blender ID")
}

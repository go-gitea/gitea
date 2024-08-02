// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"time"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

type fakeProvider struct{}

func (p *fakeProvider) Name() string {
	return "fake"
}

func (p *fakeProvider) SetName(name string) {}

func (p *fakeProvider) BeginAuth(state string) (goth.Session, error) {
	return nil, nil
}

func (p *fakeProvider) UnmarshalSession(string) (goth.Session, error) {
	return nil, nil
}

func (p *fakeProvider) FetchUser(goth.Session) (goth.User, error) {
	return goth.User{}, nil
}

func (p *fakeProvider) Debug(bool) {
}

func (p *fakeProvider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	switch refreshToken {
	case "expired":
		return nil, &oauth2.RetrieveError{
			ErrorCode: "invalid_grant",
		}
	default:
		return &oauth2.Token{
			AccessToken:  "token",
			TokenType:    "Bearer",
			RefreshToken: "refresh",
			Expiry:       time.Now().Add(time.Hour),
		}, nil
	}
}

func (p *fakeProvider) RefreshTokenAvailable() bool {
	return true
}

func init() {
	RegisterGothProvider(
		NewSimpleProvider("fake", "Fake", []string{"account"},
			func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
				return &fakeProvider{}
			}))
}

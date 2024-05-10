// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package blenderid_test

import (
	"os"
	"testing"

	"code.gitea.io/gitea/services/auth/source/oauth2/blenderid"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/assert"
)

func Test_New(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	p := provider()

	a.Equal(p.ClientKey, os.Getenv("BLENDERID_KEY"))
	a.Equal(p.Secret, os.Getenv("BLENDERID_SECRET"))
	a.Equal(p.CallbackURL, "/foo")
}

func Test_NewCustomisedURL(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	p := urlCustomisedURLProvider()
	session, err := p.BeginAuth("test_state")
	s := session.(*blenderid.Session)
	a.NoError(err)
	a.Contains(s.AuthURL, "http://authURL")
}

func Test_Implements_Provider(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	a.Implements((*goth.Provider)(nil), provider())
}

func Test_BeginAuth(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	p := provider()
	session, err := p.BeginAuth("test_state")
	s := session.(*blenderid.Session)
	a.NoError(err)
	a.Contains(s.AuthURL, "id.blender.org/oauth/authorize")
}

func Test_SessionFromJSON(t *testing.T) {
	t.Parallel()
	a := assert.New(t)

	p := provider()
	session, err := p.UnmarshalSession(`{"AuthURL":"https://id.blender.org/oauth/authorize","AccessToken":"1234567890"}`)
	a.NoError(err)

	s := session.(*blenderid.Session)
	a.Equal(s.AuthURL, "https://id.blender.org/oauth/authorize")
	a.Equal(s.AccessToken, "1234567890")
}

func provider() *blenderid.Provider {
	return blenderid.New(os.Getenv("BLENDERID_KEY"), os.Getenv("BLENDERID_SECRET"), "/foo")
}

func urlCustomisedURLProvider() *blenderid.Provider {
	return blenderid.NewCustomisedURL(os.Getenv("BLENDERID_KEY"), os.Getenv("BLENDERID_SECRET"), "/foo", "http://authURL", "http://tokenURL", "http://profileURL")
}

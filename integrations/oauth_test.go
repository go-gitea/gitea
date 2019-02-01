package integrations

import (
	"github.com/stretchr/testify/assert"
	"testing"
)
const defaultAuthorize = "/login/oauth/authorize?client_id=da7da3ba-9a13-4167-856f-3899de0b0138&redirect_uri=a&response_type=code&state=thestate"

func TestNoClientID(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/login/oauth/authorize")
	ctx := loginUser(t, "user2")
	ctx.MakeRequest(t, req, 500)
}

func TestLoginRedirect(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", "/login/oauth/authorize")
	assert.Contains(t, MakeRequest(t, req, 302).Body.String(), "/user/login")
}

func TestShowAuthorize(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", defaultAuthorize)
	ctx := loginUser(t, "user4")
	resp := ctx.MakeRequest(t, req, 200)

	htmlDoc := NewHTMLParser(t, resp.Body)
	htmlDoc.AssertElement(t, "#authorize-app", true)
	htmlDoc.GetCSRF()
}

func TestRedirectWithExistingGrant(t *testing.T) {
	prepareTestEnv(t)
	req := NewRequest(t, "GET", defaultAuthorize)
	ctx := loginUser(t, "user1")
	resp := ctx.MakeRequest(t, req, 302)
	u, err := resp.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "thestate", u.Query().Get("state"))
	assert.Truef(t, len(u.Query().Get("code")) > 30, "authorization code '%s' should be longer then 30", u.Query().Get("code"))
}


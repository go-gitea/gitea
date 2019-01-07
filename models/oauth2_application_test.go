package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuth2AuthorizationCode_ValidateCodeChallenge(t *testing.T) {
	// test plain
	code := &OAuth2AuthorizationCode{
		CodeChallengeMethod: "plain",
		CodeChallenge: "test123",
	}
	assert.True(t, code.ValidateCodeChallenge("test123"))
	assert.False(t, code.ValidateCodeChallenge("ierwgjoergjio"))

	// test S256
	code = &OAuth2AuthorizationCode{
		CodeChallengeMethod: "S256",
		CodeChallenge: "CjvyTLSdR47G5zYenDA-eDWW4lRrO8yvjcWwbD_deOg",
	}
	assert.True(t, code.ValidateCodeChallenge("N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt"))
	assert.False(t, code.ValidateCodeChallenge("wiogjerogorewngoenrgoiuenorg"))

	// test unknown
	code = &OAuth2AuthorizationCode{
		CodeChallengeMethod: "monkey",
		CodeChallenge: "foiwgjioriogeiogjerger",
	}
	assert.False(t, code.ValidateCodeChallenge("foiwgjioriogeiogjerger"))

	// test no code challenge
	code = &OAuth2AuthorizationCode{
		CodeChallengeMethod: "",
		CodeChallenge: "foierjiogerogerg",
	}
	assert.True(t, code.ValidateCodeChallenge(""))
}

func TestOAuth2Application_PrimaryRedirectURI(t *testing.T) {
	app := &OAuth2Application{
		RedirectURIs: []string{"a", "b", "c"},
	}
	assert.Equal(t, "a", app.PrimaryRedirectURI())
}

func TestOAuth2Application_GenerateClientSecret(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1}).(*OAuth2Application)
	secret, err := app.GenerateClientSecret()
	assert.NoError(t, err)
	assert.True(t, len(secret) > 0)
	AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1, ClientSecret: app.ClientSecret})
}

func BenchmarkOAuth2Application_GenerateClientSecret(b *testing.B) {
	assert.NoError(b, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(b, &OAuth2Application{ID: 1}).(*OAuth2Application)
	for i := 0; i < b.N; i++ {
		_, _ = app.GenerateClientSecret()
	}
}

func TestOAuth2Application_ContainsRedirectURI(t *testing.T) {
	app := &OAuth2Application{
		RedirectURIs: []string{"a", "b", "c"},
	}
	assert.True(t, app.ContainsRedirectURI("a"))
	assert.True(t, app.ContainsRedirectURI("b"))
	assert.True(t, app.ContainsRedirectURI("c"))
	assert.False(t, app.ContainsRedirectURI("d"))
}

func TestOAuth2Application_ValidateClientSecret(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1}).(*OAuth2Application)
	secret, err := app.GenerateClientSecret()
	assert.NoError(t, err)
	assert.True(t, app.ValidateClientSecret([]byte(secret)))
	assert.False(t, app.ValidateClientSecret([]byte("fewijfowejgfiowjeoifew")))
}

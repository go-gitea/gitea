package application

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"github.com/stretchr/testify/assert"
)

func TestJWTToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	app, err := GetAppByClientID(t.Context(), "b4732d48-a71d-4f80-9d9d-82dcfd77049c")
	assert.NoError(t, err)
	assert.EqualValues(t, int64(43), app.ID)

	key, err := GenerateKeyPair()
	assert.NoError(t, err)
	assert.NotEmpty(t, key)

	pubKey, err := key.PublicKeyPEM()
	assert.NoError(t, err)

	err = app.AddJWTPublicKey(t.Context(), pubKey)
	assert.NoError(t, err)

	jwtStr, err := CreateJWTToken(key.PrivateKey, app.AppExternalData().ClientID, 600)
	assert.NoError(t, err)
	assert.NotEmpty(t, jwtStr)

	sign, err := ValidateJWTSignature(t.Context(), jwtStr)
	assert.NoError(t, err)
	assert.Equal(t, app.ID, sign.App.ID)

	jwtStr += "_invalid"
	_, err = ValidateJWTSignature(t.Context(), jwtStr)
	assert.Contains(t, err.Error(), "token signature is invalid")
}

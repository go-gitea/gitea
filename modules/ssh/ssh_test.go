package ssh_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenKeyPair(t *testing.T) {
	testCases := []struct {
		keyType      ssh.KeyType
		expectedType any
	}{
		{
			keyType:      ssh.RSA,
			expectedType: &rsa.PrivateKey{},
		},
		{
			keyType:      ssh.ED25519,
			expectedType: ed25519.PrivateKey{},
		},
		{
			keyType:      ssh.ECDSA,
			expectedType: &ecdsa.PrivateKey{},
		},
	}
	for _, tC := range testCases {
		t.Run("Generate"+string(tC.keyType), func(t *testing.T) {
			path := t.TempDir() + "/gitea." + string(tC.keyType)
			require.NoError(t, ssh.GenKeyPair(path, tC.keyType))

			file, err := os.Open(path)
			require.NoError(t, err)

			bytes, err := io.ReadAll(file)
			require.NoError(t, err)

			block, _ := pem.Decode(bytes)
			require.NotNil(t, block)
			assert.Equal(t, "PRIVATE KEY", block.Type)

			privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			require.NoError(t, err)
			assert.IsType(t, tC.expectedType, privateKey)
		})
	}
}

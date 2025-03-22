package ssh_test

import (
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
	path := t.TempDir() + "/gitea.rsa"
	require.NoError(t, ssh.GenKeyPair(path, ssh.RSA))

	file, err := os.Open(path)
	require.NoError(t, err)

	bytes, err := io.ReadAll(file)
	require.NoError(t, err)

	block, _ := pem.Decode(bytes)
	require.NotNil(t, block)
	assert.Equal(t, "PRIVATE KEY", block.Type)

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	assert.IsType(t, &rsa.PrivateKey{}, privateKey)
}

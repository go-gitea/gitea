package ssh_test

import (
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
	ssh.GenKeyPair(path)

	file, err := os.Open(path)
	require.NoError(t, err)

	bytes, err := io.ReadAll(file)
	require.NoError(t, err)

	block, _ := pem.Decode(bytes)
	require.NotNil(t, block)
	assert.Equal(t, "RSA PRIVATE KEY", block.Type)

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err)
	assert.NotNil(t, privateKey)
}

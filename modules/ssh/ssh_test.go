// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenKeyPair(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "test_host_key")

	require.NoError(t, GenKeyPair(keyPath))

	info, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	_, err = os.Stat(keyPath + ".pub")
	require.NoError(t, err)

	privPEM, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	block, rest := pem.Decode(privPEM)
	require.NotNil(t, block, "expected PEM block in private key file")
	assert.Empty(t, rest, "unexpected trailing data in private key file")
	assert.Equal(t, "PRIVATE KEY", block.Type)

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	_, ok := key.(ed25519.PrivateKey)
	assert.True(t, ok, "expected Ed25519 private key, got %T", key)
}

func TestAddHostKey_RSARestrictedToSHA2(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "rsa_host_key")

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	require.NoError(t, err)
	f, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes}))
	require.NoError(t, f.Close())

	srv := &ssh.Server{}
	require.NoError(t, addHostKey(srv, keyPath))
	require.Len(t, srv.HostSigners, 1)

	mas, ok := srv.HostSigners[0].(gossh.MultiAlgorithmSigner)
	require.True(t, ok, "expected MultiAlgorithmSigner for RSA key")

	algos := mas.Algorithms()
	assert.Contains(t, algos, gossh.KeyAlgoRSASHA256)
	assert.Contains(t, algos, gossh.KeyAlgoRSASHA512)
	assert.NotContains(t, algos, gossh.KeyAlgoRSA, "ssh-rsa (SHA-1) must not be advertised")
}

func TestAddHostKey_Ed25519NotRestricted(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ed25519_host_key")
	require.NoError(t, GenKeyPair(keyPath))

	srv := &ssh.Server{}
	require.NoError(t, addHostKey(srv, keyPath))
	require.Len(t, srv.HostSigners, 1)

	assert.Equal(t, gossh.KeyAlgoED25519, srv.HostSigners[0].PublicKey().Type())
}

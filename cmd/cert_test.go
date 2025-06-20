// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertCommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "RSA cert generation",
			args: []string{
				"cert-test",
				"--host", "localhost",
				"--rsa-bits", "2048",
				"--duration", "1h",
				"--start-date", "Jan 1 00:00:00 2024",
			},
		},
		{
			name: "ECDSA cert generation",
			args: []string{
				"cert-test",
				"--host", "localhost",
				"--ecdsa-curve", "P256",
				"--duration", "1h",
				"--start-date", "Jan 1 00:00:00 2024",
			},
		},
		{
			name: "mixed host, certificate authority",
			args: []string{
				"cert-test",
				"--host", "localhost,127.0.0.1",
				"--duration", "1h",
				"--start-date", "Jan 1 00:00:00 2024",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := cmdCert()
			tempDir := t.TempDir()

			certFile := filepath.Join(tempDir, "cert.pem")
			keyFile := filepath.Join(tempDir, "key.pem")

			err := app.Run(t.Context(), append(c.args, "--out", certFile, "--keyout", keyFile))
			require.NoError(t, err)

			assert.FileExists(t, certFile)
			assert.FileExists(t, keyFile)
		})
	}
}

func TestCertCommandFailures(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		errMsg string
	}{
		{
			name: "Start Date Parsing failure",
			args: []string{
				"cert-test",
				"--host", "localhost",
				"--start-date", "invalid-date",
			},
			errMsg: "parsing time",
		},
		{
			name: "Unknown curve",
			args: []string{
				"cert-test",
				"--host", "localhost",
				"--ecdsa-curve", "invalid-curve",
			},
			errMsg: "unrecognized elliptic curve",
		},
		{
			name: "Key generation failure",
			args: []string{
				"cert-test",
				"--host", "localhost",
				"--rsa-bits", "invalid-bits",
			},
		},
		{
			name: "Missing parameters",
			args: []string{
				"cert-test",
			},
			errMsg: `"host" not set`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := cmdCert()
			tempDir := t.TempDir()

			certFile := filepath.Join(tempDir, "cert.pem")
			keyFile := filepath.Join(tempDir, "key.pem")
			err := app.Run(t.Context(), append(c.args, "--out", certFile, "--keyout", keyFile))
			require.Error(t, err)
			if c.errMsg != "" {
				assert.ErrorContains(t, err, c.errMsg)
			}
			assert.NoFileExists(t, certFile)
			assert.NoFileExists(t, keyFile)
		})
	}
}

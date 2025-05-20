// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/services/auth/source/smtp"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestAddSMTP(t *testing.T) {
	testCases := []struct {
		name   string
		args   []string
		source *auth_model.Source
		errMsg string
	}{
		{
			name: "missing name",
			args: []string{
				"--host", "localhost",
				"--port", "25",
			},
			errMsg: "name must be set",
		},
		{
			name: "missing host",
			args: []string{
				"--name", "test",
				"--port", "25",
			},
			errMsg: "host must be set",
		},
		{
			name: "missing port",
			args: []string{
				"--name", "test",
				"--host", "localhost",
			},
			errMsg: "port must be set",
		},
		{
			name: "valid config",
			args: []string{
				"--name", "test",
				"--host", "localhost",
				"--port", "25",
			},
			source: &auth_model.Source{
				Type:     auth_model.SMTP,
				Name:     "test",
				IsActive: true,
				Cfg: &smtp.Source{
					Auth: "PLAIN",
					Host: "localhost",
					Port: 25,
					// ForceSMTPS: true,
					// SkipVerify: true,
				},
				TwoFactorPolicy: "skip",
			},
		},
		{
			name: "valid config with options",
			args: []string{
				"--name", "test",
				"--host", "localhost",
				"--port", "25",
				"--auth-type", "LOGIN",
				"--force-smtps=false",
				"--skip-verify=false",
				"--helo-hostname", "example.com",
				"--disable-helo=false",
				"--allowed-domains", "example.com,example.org",
				"--skip-local-2fa=false",
				"--active=false",
			},
			source: &auth_model.Source{
				Type:     auth_model.SMTP,
				Name:     "test",
				IsActive: false,
				Cfg: &smtp.Source{
					Auth:           "LOGIN",
					Host:           "localhost",
					Port:           25,
					ForceSMTPS:     false,
					SkipVerify:     false,
					HeloHostname:   "example.com",
					DisableHelo:    false,
					AllowedDomains: "example.com,example.org",
				},
				TwoFactorPolicy: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := &authService{
				initDB: func(ctx context.Context) error {
					return nil
				},
				createAuthSource: func(ctx context.Context, source *auth_model.Source) error {
					assert.Equal(t, tc.source, source)
					return nil
				},
			}

			cmd := &cli.Command{
				Flags:  microcmdAuthAddSMTP().Flags,
				Action: a.runAddSMTP,
			}

			args := []string{"smtp-test"}
			args = append(args, tc.args...)

			t.Log(args)
			err := cmd.Run(t.Context(), args)

			if tc.errMsg != "" {
				assert.EqualError(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateSMTP(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string
		existingAuthSource *auth_model.Source
		authSource         *auth_model.Source
		errMsg             string
	}{
		{
			name: "missing id",
			args: []string{
				"--name", "test",
				"--host", "localhost",
				"--port", "25",
			},
			errMsg: "--id flag is missing",
		},
		{
			name: "valid config",
			existingAuthSource: &auth_model.Source{
				ID:       1,
				Type:     auth_model.SMTP,
				Name:     "old name",
				IsActive: true,
				Cfg: &smtp.Source{
					Auth:       "PLAIN",
					Host:       "old host",
					Port:       26,
					ForceSMTPS: true,
					SkipVerify: true,
				},
				TwoFactorPolicy: "",
			},
			args: []string{
				"--id", "1",
				"--name", "test",
				"--host", "localhost",
				"--port", "25",
			},
			authSource: &auth_model.Source{
				ID:       1,
				Type:     auth_model.SMTP,
				Name:     "test",
				IsActive: true,
				Cfg: &smtp.Source{
					Auth:       "PLAIN",
					Host:       "localhost",
					Port:       25,
					ForceSMTPS: true,
					SkipVerify: true,
				},
				TwoFactorPolicy: "skip",
			},
		},
		{
			name: "valid config with options",
			existingAuthSource: &auth_model.Source{
				ID:       1,
				Type:     auth_model.SMTP,
				Name:     "old name",
				IsActive: true,
				Cfg: &smtp.Source{
					Auth:           "PLAIN",
					Host:           "old host",
					Port:           26,
					ForceSMTPS:     true,
					SkipVerify:     true,
					HeloHostname:   "old.example.com",
					DisableHelo:    false,
					AllowedDomains: "old.example.com",
				},
				TwoFactorPolicy: "",
			},
			args: []string{
				"--id", "1",
				"--name", "test",
				"--host", "localhost",
				"--port", "25",
				"--auth-type", "LOGIN",
				"--force-smtps=false",
				"--skip-verify=false",
				"--helo-hostname", "example.com",
				"--disable-helo=true",
				"--allowed-domains", "example.com,example.org",
				"--skip-local-2fa=true",
				"--active=false",
			},
			authSource: &auth_model.Source{
				ID:       1,
				Type:     auth_model.SMTP,
				Name:     "test",
				IsActive: false,
				Cfg: &smtp.Source{
					Auth:           "LOGIN",
					Host:           "localhost",
					Port:           25,
					ForceSMTPS:     false,
					SkipVerify:     false,
					HeloHostname:   "example.com",
					DisableHelo:    true,
					AllowedDomains: "example.com,example.org",
				},
				TwoFactorPolicy: "skip",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := &authService{
				initDB: func(ctx context.Context) error {
					return nil
				},
				getAuthSourceByID: func(ctx context.Context, id int64) (*auth_model.Source, error) {
					return &auth_model.Source{
						ID:       1,
						Type:     auth_model.SMTP,
						Name:     "test",
						IsActive: true,
						Cfg: &smtp.Source{
							Auth:       "PLAIN",
							SkipVerify: true,
							ForceSMTPS: true,
						},
						TwoFactorPolicy: "skip",
					}, nil
				},

				updateAuthSource: func(ctx context.Context, source *auth_model.Source) error {
					assert.Equal(t, tc.authSource, source)
					return nil
				},
			}

			app := &cli.Command{
				Flags:  microcmdAuthUpdateSMTP().Flags,
				Action: a.runUpdateSMTP,
			}
			args := []string{"smtp-tests"}
			args = append(args, tc.args...)

			err := app.Run(t.Context(), args)

			if tc.errMsg != "" {
				assert.EqualError(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

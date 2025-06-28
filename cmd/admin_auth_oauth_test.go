// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestAddOauth(t *testing.T) {
	testCases := []struct {
		name   string
		args   []string
		source *auth_model.Source
		errMsg string
	}{
		{
			name: "valid config",
			args: []string{
				"--name", "test",
				"--provider", "github",
				"--key", "some_key",
				"--secret", "some_secret",
			},
			source: &auth_model.Source{
				Type:     auth_model.OAuth2,
				Name:     "test",
				IsActive: true,
				Cfg: &oauth2.Source{
					Scopes:       []string{},
					Provider:     "github",
					ClientID:     "some_key",
					ClientSecret: "some_secret",
				},
				TwoFactorPolicy: "",
			},
		},
		{
			name: "valid config with openid connect",
			args: []string{
				"--name", "test",
				"--provider", "openidConnect",
				"--key", "some_key",
				"--secret", "some_secret",
				"--auto-discover-url", "https://example.com",
			},
			source: &auth_model.Source{
				Type:     auth_model.OAuth2,
				Name:     "test",
				IsActive: true,
				Cfg: &oauth2.Source{
					Scopes:                        []string{},
					Provider:                      "openidConnect",
					ClientID:                      "some_key",
					ClientSecret:                  "some_secret",
					OpenIDConnectAutoDiscoveryURL: "https://example.com",
				},
				TwoFactorPolicy: "",
			},
		},
		{
			name: "valid config with options",
			args: []string{
				"--name", "test",
				"--provider", "gitlab",
				"--key", "some_key",
				"--secret", "some_secret",
				"--use-custom-urls", "true",
				"--custom-token-url", "https://example.com/token",
				"--custom-auth-url", "https://example.com/auth",
				"--custom-profile-url", "https://example.com/profile",
				"--custom-email-url", "https://example.com/email",
				"--custom-tenant-id", "some_tenant",
				"--icon-url", "https://example.com/icon",
				"--scopes", "scope1,scope2",
				"--skip-local-2fa", "true",
				"--required-claim-name", "claim_name",
				"--required-claim-value", "claim_value",
				"--group-claim-name", "group_name",
				"--admin-group", "admin",
				"--restricted-group", "restricted",
				"--group-team-map", `{"group1": [1,2]}`,
				"--group-team-map-removal=true",
			},
			source: &auth_model.Source{
				Type:     auth_model.OAuth2,
				Name:     "test",
				IsActive: true,
				Cfg: &oauth2.Source{
					Provider:     "gitlab",
					ClientID:     "some_key",
					ClientSecret: "some_secret",
					CustomURLMapping: &oauth2.CustomURLMapping{
						TokenURL:   "https://example.com/token",
						AuthURL:    "https://example.com/auth",
						ProfileURL: "https://example.com/profile",
						EmailURL:   "https://example.com/email",
						Tenant:     "some_tenant",
					},
					IconURL:             "https://example.com/icon",
					Scopes:              []string{"scope1", "scope2"},
					RequiredClaimName:   "claim_name",
					RequiredClaimValue:  "claim_value",
					GroupClaimName:      "group_name",
					AdminGroup:          "admin",
					RestrictedGroup:     "restricted",
					GroupTeamMap:        `{"group1": [1,2]}`,
					GroupTeamMapRemoval: true,
				},
				TwoFactorPolicy: "skip",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var createdSource *auth_model.Source
			a := &authService{
				initDB: func(ctx context.Context) error {
					return nil
				},
				createAuthSource: func(ctx context.Context, source *auth_model.Source) error {
					createdSource = source
					return nil
				},
			}

			app := &cli.Command{
				Flags:  microcmdAuthAddOauth().Flags,
				Action: a.runAddOauth,
			}

			args := []string{"oauth-test"}
			args = append(args, tc.args...)

			err := app.Run(t.Context(), args)

			if tc.errMsg != "" {
				assert.EqualError(t, err, tc.errMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.source, createdSource)
			}
		})
	}
}

func TestUpdateOauth(t *testing.T) {
	testCases := []struct {
		name               string
		args               []string
		id                 int64
		existingAuthSource *auth_model.Source
		authSource         *auth_model.Source
		errMsg             string
	}{
		{
			name: "missing id",
			args: []string{
				"--name", "test",
			},
			errMsg: "--id flag is missing",
		},
		{
			name: "valid config",
			id:   1,
			existingAuthSource: &auth_model.Source{
				ID:       1,
				Type:     auth_model.OAuth2,
				Name:     "old name",
				IsActive: true,
				Cfg: &oauth2.Source{
					Provider:     "github",
					ClientID:     "old_key",
					ClientSecret: "old_secret",
				},
				TwoFactorPolicy: "",
			},
			args: []string{
				"--id", "1",
				"--name", "test",
				"--provider", "gitlab",
				"--key", "new_key",
				"--secret", "new_secret",
			},
			authSource: &auth_model.Source{
				ID:       1,
				Type:     auth_model.OAuth2,
				Name:     "test",
				IsActive: true,
				Cfg: &oauth2.Source{
					Provider:         "gitlab",
					ClientID:         "new_key",
					ClientSecret:     "new_secret",
					CustomURLMapping: &oauth2.CustomURLMapping{},
				},
				TwoFactorPolicy: "",
			},
		},
		{
			name: "valid config with options",
			id:   1,
			existingAuthSource: &auth_model.Source{
				ID:       1,
				Type:     auth_model.OAuth2,
				Name:     "old name",
				IsActive: true,
				Cfg: &oauth2.Source{
					Provider:     "gitlab",
					ClientID:     "old_key",
					ClientSecret: "old_secret",
					CustomURLMapping: &oauth2.CustomURLMapping{
						TokenURL:   "https://old.example.com/token",
						AuthURL:    "https://old.example.com/auth",
						ProfileURL: "https://old.example.com/profile",
						EmailURL:   "https://old.example.com/email",
						Tenant:     "old_tenant",
					},
					IconURL:             "https://old.example.com/icon",
					Scopes:              []string{"old_scope1", "old_scope2"},
					RequiredClaimName:   "old_claim_name",
					RequiredClaimValue:  "old_claim_value",
					GroupClaimName:      "old_group_name",
					AdminGroup:          "old_admin",
					RestrictedGroup:     "old_restricted",
					GroupTeamMap:        `{"old_group1": [1,2]}`,
					GroupTeamMapRemoval: true,
				},
				TwoFactorPolicy: "",
			},
			args: []string{
				"--id", "1",
				"--name", "test",
				"--provider", "github",
				"--key", "new_key",
				"--secret", "new_secret",
				"--use-custom-urls", "true",
				"--custom-token-url", "https://example.com/token",
				"--custom-auth-url", "https://example.com/auth",
				"--custom-profile-url", "https://example.com/profile",
				"--custom-email-url", "https://example.com/email",
				"--custom-tenant-id", "new_tenant",
				"--icon-url", "https://example.com/icon",
				"--scopes", "scope1,scope2",
				"--skip-local-2fa=true",
				"--required-claim-name", "claim_name",
				"--required-claim-value", "claim_value",
				"--group-claim-name", "group_name",
				"--admin-group", "admin",
				"--restricted-group", "restricted",
				"--group-team-map", `{"group1": [1,2]}`,
				"--group-team-map-removal=false",
			},
			authSource: &auth_model.Source{
				ID:       1,
				Type:     auth_model.OAuth2,
				Name:     "test",
				IsActive: true,
				Cfg: &oauth2.Source{
					Provider:     "github",
					ClientID:     "new_key",
					ClientSecret: "new_secret",
					CustomURLMapping: &oauth2.CustomURLMapping{
						TokenURL:   "https://example.com/token",
						AuthURL:    "https://example.com/auth",
						ProfileURL: "https://example.com/profile",
						EmailURL:   "https://example.com/email",
						Tenant:     "new_tenant",
					},
					IconURL:             "https://example.com/icon",
					Scopes:              []string{"scope1", "scope2"},
					RequiredClaimName:   "claim_name",
					RequiredClaimValue:  "claim_value",
					GroupClaimName:      "group_name",
					AdminGroup:          "admin",
					RestrictedGroup:     "restricted",
					GroupTeamMap:        `{"group1": [1,2]}`,
					GroupTeamMapRemoval: false,
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
						Type:     auth_model.OAuth2,
						Name:     "test",
						IsActive: true,
						Cfg: &oauth2.Source{
							CustomURLMapping: &oauth2.CustomURLMapping{},
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
				Flags:  microcmdAuthUpdateOauth().Flags,
				Action: a.runUpdateOauth,
			}

			args := []string{"oauth-test"}
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

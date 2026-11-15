// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/util"
	"gitea.dev/services/auth/source/oauth2"

	"github.com/urfave/cli/v3"
)

func oauthCLIFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Value: "",
			Usage: "Application Name",
		},
		&cli.StringFlag{
			Name:  "provider",
			Value: "",
			Usage: "OAuth2 Provider",
		},
		&cli.StringFlag{
			Name:  "key",
			Value: "",
			Usage: "Client ID (Key)",
		},
		&cli.StringFlag{
			Name:  "secret",
			Value: "",
			Usage: "Client Secret (visible in the process arguments; prefer --secret-file)",
		},
		&cli.StringFlag{
			Name:  "secret-file",
			Usage: "Read the Client Secret from an absolute regular file (owner-only permissions on Unix)",
		},
		&cli.StringFlag{
			Name:  "auto-discover-url",
			Value: "",
			Usage: "OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider)",
		},
		&cli.StringFlag{
			Name:  "use-custom-urls",
			Value: "false",
			Usage: "Use custom URLs for GitLab/GitHub OAuth endpoints",
		},
		&cli.StringFlag{
			Name:  "custom-tenant-id",
			Value: "",
			Usage: "Use custom Tenant ID for OAuth endpoints",
		},
		&cli.StringFlag{
			Name:  "custom-auth-url",
			Value: "",
			Usage: "Use a custom Authorization URL (option for GitLab/GitHub)",
		},
		&cli.StringFlag{
			Name:  "custom-token-url",
			Value: "",
			Usage: "Use a custom Token URL (option for GitLab/GitHub)",
		},
		&cli.StringFlag{
			Name:  "custom-profile-url",
			Value: "",
			Usage: "Use a custom Profile URL (option for GitLab/GitHub)",
		},
		&cli.StringFlag{
			Name:  "custom-email-url",
			Value: "",
			Usage: "Use a custom Email URL (option for GitHub)",
		},
		&cli.StringFlag{
			Name:  "icon-url",
			Value: "",
			Usage: "Custom icon URL for OAuth2 login source",
		},
		&cli.BoolFlag{
			Name:  "skip-local-2fa",
			Usage: "Set to true to skip local 2fa for users authenticated by this source",
		},
		&cli.StringSliceFlag{
			Name:  "scopes",
			Value: nil,
			Usage: "Scopes to request when to authenticate against this OAuth2 source",
		},
		&cli.StringFlag{
			Name:  "ssh-public-key-claim-name",
			Usage: "Claim name that provides SSH public keys",
		},
		&cli.StringFlag{
			Name:  "full-name-claim-name",
			Usage: "Claim name that provides user's full name",
		},
		&cli.StringFlag{
			Name:  "required-claim-name",
			Value: "",
			Usage: "Claim name that has to be set to allow users to login with this source",
		},
		&cli.StringFlag{
			Name:  "required-claim-value",
			Value: "",
			Usage: "Claim value that has to be set to allow users to login with this source",
		},
		&cli.StringFlag{
			Name:  "group-claim-name",
			Value: "",
			Usage: "Claim name providing group names for this source",
		},
		&cli.StringFlag{
			Name:  "admin-group",
			Value: "",
			Usage: "Group Claim value for administrator users",
		},
		&cli.StringFlag{
			Name:  "restricted-group",
			Value: "",
			Usage: "Group Claim value for restricted users",
		},
		&cli.StringFlag{
			Name:  "group-team-map",
			Value: "",
			Usage: "JSON mapping between groups and org teams",
		},
		&cli.BoolFlag{
			Name:  "group-team-map-removal",
			Usage: "Activate automatic team membership removal depending on groups",
		},
	}
}

func microcmdAuthAddOauth() *cli.Command {
	return &cli.Command{
		Name:  "add-oauth",
		Usage: "Add new Oauth authentication source",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return newAuthService().runAddOauth(ctx, cmd)
		},
		Flags: oauthCLIFlags(),
	}
}

func microcmdAuthUpdateOauth() *cli.Command {
	return &cli.Command{
		Name:  "update-oauth",
		Usage: "Update existing Oauth authentication source",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return newAuthService().runUpdateOauth(ctx, cmd)
		},
		Flags: append(oauthCLIFlags()[:1], append([]cli.Flag{&cli.Int64Flag{
			Name:  "id",
			Usage: "ID of authentication source",
		}}, oauthCLIFlags()[1:]...)...),
	}
}

func readOAuthClientSecret(c *cli.Command) (string, error) {
	const maxSize = 64 * 1024

	if c.IsSet("secret") && c.IsSet("secret-file") {
		return "", errors.New("--secret and --secret-file cannot be used together")
	}
	if !c.IsSet("secret-file") {
		return c.String("secret"), nil
	}

	path := c.String("secret-file")
	if !filepath.IsAbs(path) {
		return "", errors.New("--secret-file must be an absolute path")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("stat client secret file: %w", err)
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0) {
		return "", errors.New("client secret file must be a regular file accessible only to its owner")
	}
	if info.Size() > maxSize {
		return "", errors.New("client secret file is too large")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open client secret file: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat opened client secret file: %w", err)
	}
	if !os.SameFile(info, openedInfo) {
		return "", errors.New("client secret file changed while it was opened")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return "", fmt.Errorf("read client secret file: %w", err)
	}
	if len(data) > maxSize {
		return "", errors.New("client secret file is too large")
	}
	secret := strings.TrimSuffix(string(data), "\n")
	secret = strings.TrimSuffix(secret, "\r")
	if secret == "" || strings.ContainsAny(secret, "\r\n") {
		return "", errors.New("client secret file must contain one non-empty line")
	}
	return secret, nil
}

func parseOAuth2Config(c *cli.Command, clientSecret string) *oauth2.Source {
	var customURLMapping *oauth2.CustomURLMapping
	if c.IsSet("use-custom-urls") {
		customURLMapping = &oauth2.CustomURLMapping{
			TokenURL:   c.String("custom-token-url"),
			AuthURL:    c.String("custom-auth-url"),
			ProfileURL: c.String("custom-profile-url"),
			EmailURL:   c.String("custom-email-url"),
			Tenant:     c.String("custom-tenant-id"),
		}
	} else {
		customURLMapping = nil
	}
	return &oauth2.Source{
		Provider:                      c.String("provider"),
		ClientID:                      c.String("key"),
		ClientSecret:                  clientSecret,
		OpenIDConnectAutoDiscoveryURL: c.String("auto-discover-url"),
		CustomURLMapping:              customURLMapping,
		IconURL:                       c.String("icon-url"),
		Scopes:                        c.StringSlice("scopes"),
		RequiredClaimName:             c.String("required-claim-name"),
		RequiredClaimValue:            c.String("required-claim-value"),
		GroupClaimName:                c.String("group-claim-name"),
		AdminGroup:                    c.String("admin-group"),
		RestrictedGroup:               c.String("restricted-group"),
		GroupTeamMap:                  c.String("group-team-map"),
		GroupTeamMapRemoval:           c.Bool("group-team-map-removal"),
		SSHPublicKeyClaimName:         c.String("ssh-public-key-claim-name"),
		FullNameClaimName:             c.String("full-name-claim-name"),
	}
}

func (a *authService) runAddOauth(ctx context.Context, c *cli.Command) error {
	if err := a.initDB(ctx); err != nil {
		return err
	}

	clientSecret, err := readOAuthClientSecret(c)
	if err != nil {
		return err
	}
	config := parseOAuth2Config(c, clientSecret)
	if config.Provider == "openidConnect" {
		discoveryURL, err := url.Parse(config.OpenIDConnectAutoDiscoveryURL)
		if err != nil || (discoveryURL.Scheme != "http" && discoveryURL.Scheme != "https") {
			return fmt.Errorf("invalid Auto Discovery URL: %s (this must be a valid URL starting with http:// or https://)", config.OpenIDConnectAutoDiscoveryURL)
		}
	}

	return a.createAuthSource(ctx, &auth_model.Source{
		Type:            auth_model.OAuth2,
		Name:            c.String("name"),
		IsActive:        true,
		Cfg:             config,
		TwoFactorPolicy: util.Iif(c.Bool("skip-local-2fa"), "skip", ""),
	})
}

func (a *authService) runUpdateOauth(ctx context.Context, c *cli.Command) error {
	if !c.IsSet("id") {
		return errors.New("--id flag is missing")
	}

	if err := a.initDB(ctx); err != nil {
		return err
	}

	source, err := a.getAuthSourceOfType(ctx, c.Int64("id"), auth_model.OAuth2)
	if err != nil {
		return err
	}
	oAuth2Config := auth_model.MustSourceCfg[*oauth2.Source](source)
	if c.IsSet("secret") || c.IsSet("secret-file") {
		clientSecret, err := readOAuthClientSecret(c)
		if err != nil {
			return err
		}
		oAuth2Config.ClientSecret = clientSecret
	}

	if c.IsSet("name") {
		source.Name = c.String("name")
	}

	if c.IsSet("provider") {
		oAuth2Config.Provider = c.String("provider")
	}

	if c.IsSet("key") {
		oAuth2Config.ClientID = c.String("key")
	}

	if c.IsSet("auto-discover-url") {
		oAuth2Config.OpenIDConnectAutoDiscoveryURL = c.String("auto-discover-url")
	}

	if c.IsSet("icon-url") {
		oAuth2Config.IconURL = c.String("icon-url")
	}

	if c.IsSet("scopes") {
		oAuth2Config.Scopes = c.StringSlice("scopes")
	}

	if c.IsSet("required-claim-name") {
		oAuth2Config.RequiredClaimName = c.String("required-claim-name")
	}
	if c.IsSet("required-claim-value") {
		oAuth2Config.RequiredClaimValue = c.String("required-claim-value")
	}

	if c.IsSet("group-claim-name") {
		oAuth2Config.GroupClaimName = c.String("group-claim-name")
	}
	if c.IsSet("admin-group") {
		oAuth2Config.AdminGroup = c.String("admin-group")
	}
	if c.IsSet("restricted-group") {
		oAuth2Config.RestrictedGroup = c.String("restricted-group")
	}
	if c.IsSet("group-team-map") {
		oAuth2Config.GroupTeamMap = c.String("group-team-map")
	}
	if c.IsSet("group-team-map-removal") {
		oAuth2Config.GroupTeamMapRemoval = c.Bool("group-team-map-removal")
	}
	if c.IsSet("ssh-public-key-claim-name") {
		oAuth2Config.SSHPublicKeyClaimName = c.String("ssh-public-key-claim-name")
	}
	if c.IsSet("full-name-claim-name") {
		oAuth2Config.FullNameClaimName = c.String("full-name-claim-name")
	}

	// update custom URL mapping
	customURLMapping := &oauth2.CustomURLMapping{}

	if oAuth2Config.CustomURLMapping != nil {
		customURLMapping.TokenURL = oAuth2Config.CustomURLMapping.TokenURL
		customURLMapping.AuthURL = oAuth2Config.CustomURLMapping.AuthURL
		customURLMapping.ProfileURL = oAuth2Config.CustomURLMapping.ProfileURL
		customURLMapping.EmailURL = oAuth2Config.CustomURLMapping.EmailURL
		customURLMapping.Tenant = oAuth2Config.CustomURLMapping.Tenant
	}
	if c.IsSet("use-custom-urls") && c.IsSet("custom-token-url") {
		customURLMapping.TokenURL = c.String("custom-token-url")
	}

	if c.IsSet("use-custom-urls") && c.IsSet("custom-auth-url") {
		customURLMapping.AuthURL = c.String("custom-auth-url")
	}

	if c.IsSet("use-custom-urls") && c.IsSet("custom-profile-url") {
		customURLMapping.ProfileURL = c.String("custom-profile-url")
	}

	if c.IsSet("use-custom-urls") && c.IsSet("custom-email-url") {
		customURLMapping.EmailURL = c.String("custom-email-url")
	}

	if c.IsSet("use-custom-urls") && c.IsSet("custom-tenant-id") {
		customURLMapping.Tenant = c.String("custom-tenant-id")
	}

	oAuth2Config.CustomURLMapping = customURLMapping
	source.Cfg = oAuth2Config
	source.TwoFactorPolicy = util.Iif(c.Bool("skip-local-2fa"), "skip", "")
	return a.updateAuthSource(ctx, source)
}

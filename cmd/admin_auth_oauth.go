// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"net/url"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"

	"github.com/urfave/cli/v2"
)

var (
	oauthCLIFlags = []cli.Flag{
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
			Usage: "Client Secret",
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

	microcmdAuthAddOauth = &cli.Command{
		Name:   "add-oauth",
		Usage:  "Add new Oauth authentication source",
		Action: runAddOauth,
		Flags:  oauthCLIFlags,
	}

	microcmdAuthUpdateOauth = &cli.Command{
		Name:   "update-oauth",
		Usage:  "Update existing Oauth authentication source",
		Action: runUpdateOauth,
		Flags:  append(oauthCLIFlags[:1], append([]cli.Flag{idFlag}, oauthCLIFlags[1:]...)...),
	}
)

func parseOAuth2Config(c *cli.Context) *oauth2.Source {
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
		ClientSecret:                  c.String("secret"),
		OpenIDConnectAutoDiscoveryURL: c.String("auto-discover-url"),
		CustomURLMapping:              customURLMapping,
		IconURL:                       c.String("icon-url"),
		SkipLocalTwoFA:                c.Bool("skip-local-2fa"),
		Scopes:                        c.StringSlice("scopes"),
		RequiredClaimName:             c.String("required-claim-name"),
		RequiredClaimValue:            c.String("required-claim-value"),
		GroupClaimName:                c.String("group-claim-name"),
		AdminGroup:                    c.String("admin-group"),
		RestrictedGroup:               c.String("restricted-group"),
		GroupTeamMap:                  c.String("group-team-map"),
		GroupTeamMapRemoval:           c.Bool("group-team-map-removal"),
	}
}

func runAddOauth(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	config := parseOAuth2Config(c)
	if config.Provider == "openidConnect" {
		discoveryURL, err := url.Parse(config.OpenIDConnectAutoDiscoveryURL)
		if err != nil || (discoveryURL.Scheme != "http" && discoveryURL.Scheme != "https") {
			return fmt.Errorf("invalid Auto Discovery URL: %s (this must be a valid URL starting with http:// or https://)", config.OpenIDConnectAutoDiscoveryURL)
		}
	}

	return auth_model.CreateSource(ctx, &auth_model.Source{
		Type:     auth_model.OAuth2,
		Name:     c.String("name"),
		IsActive: true,
		Cfg:      config,
	})
}

func runUpdateOauth(c *cli.Context) error {
	if !c.IsSet("id") {
		return fmt.Errorf("--id flag is missing")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	source, err := auth_model.GetSourceByID(ctx, c.Int64("id"))
	if err != nil {
		return err
	}

	oAuth2Config := source.Cfg.(*oauth2.Source)

	if c.IsSet("name") {
		source.Name = c.String("name")
	}

	if c.IsSet("provider") {
		oAuth2Config.Provider = c.String("provider")
	}

	if c.IsSet("key") {
		oAuth2Config.ClientID = c.String("key")
	}

	if c.IsSet("secret") {
		oAuth2Config.ClientSecret = c.String("secret")
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

	return auth_model.UpdateSource(ctx, source)
}

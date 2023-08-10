// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/auth/source/smtp"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/urfave/cli"
)

var (
	// CmdAdmin represents the available admin sub-command.
	CmdAdmin = cli.Command{
		Name:  "admin",
		Usage: "Command line interface to perform common administrative operations",
		Subcommands: []cli.Command{
			subcmdUser,
			subcmdRepoSyncReleases,
			subcmdRegenerate,
			subcmdAuth,
			subcmdSendMail,
		},
	}

	subcmdRepoSyncReleases = cli.Command{
		Name:   "repo-sync-releases",
		Usage:  "Synchronize repository releases with tags",
		Action: runRepoSyncReleases,
	}

	subcmdRegenerate = cli.Command{
		Name:  "regenerate",
		Usage: "Regenerate specific files",
		Subcommands: []cli.Command{
			microcmdRegenHooks,
			microcmdRegenKeys,
		},
	}

	microcmdRegenHooks = cli.Command{
		Name:   "hooks",
		Usage:  "Regenerate git-hooks",
		Action: runRegenerateHooks,
	}

	microcmdRegenKeys = cli.Command{
		Name:   "keys",
		Usage:  "Regenerate authorized_keys file",
		Action: runRegenerateKeys,
	}

	subcmdAuth = cli.Command{
		Name:  "auth",
		Usage: "Modify external auth providers",
		Subcommands: []cli.Command{
			microcmdAuthAddOauth,
			microcmdAuthUpdateOauth,
			cmdAuthAddLdapBindDn,
			cmdAuthUpdateLdapBindDn,
			cmdAuthAddLdapSimpleAuth,
			cmdAuthUpdateLdapSimpleAuth,
			microcmdAuthAddSMTP,
			microcmdAuthUpdateSMTP,
			microcmdAuthList,
			microcmdAuthDelete,
		},
	}

	microcmdAuthList = cli.Command{
		Name:   "list",
		Usage:  "List auth sources",
		Action: runListAuth,
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "min-width",
				Usage: "Minimal cell width including any padding for the formatted table",
				Value: 0,
			},
			cli.IntFlag{
				Name:  "tab-width",
				Usage: "width of tab characters in formatted table (equivalent number of spaces)",
				Value: 8,
			},
			cli.IntFlag{
				Name:  "padding",
				Usage: "padding added to a cell before computing its width",
				Value: 1,
			},
			cli.StringFlag{
				Name:  "pad-char",
				Usage: `ASCII char used for padding if padchar == '\\t', the Writer will assume that the width of a '\\t' in the formatted output is tabwidth, and cells are left-aligned independent of align_left (for correct-looking results, tabwidth must correspond to the tab width in the viewer displaying the result)`,
				Value: "\t",
			},
			cli.BoolFlag{
				Name:  "vertical-bars",
				Usage: "Set to true to print vertical bars between columns",
			},
		},
	}

	idFlag = cli.Int64Flag{
		Name:  "id",
		Usage: "ID of authentication source",
	}

	microcmdAuthDelete = cli.Command{
		Name:   "delete",
		Usage:  "Delete specific auth source",
		Flags:  []cli.Flag{idFlag},
		Action: runDeleteAuth,
	}

	oauthCLIFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Value: "",
			Usage: "Application Name",
		},
		cli.StringFlag{
			Name:  "provider",
			Value: "",
			Usage: "OAuth2 Provider",
		},
		cli.StringFlag{
			Name:  "key",
			Value: "",
			Usage: "Client ID (Key)",
		},
		cli.StringFlag{
			Name:  "secret",
			Value: "",
			Usage: "Client Secret",
		},
		cli.StringFlag{
			Name:  "auto-discover-url",
			Value: "",
			Usage: "OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider)",
		},
		cli.StringFlag{
			Name:  "use-custom-urls",
			Value: "false",
			Usage: "Use custom URLs for GitLab/GitHub OAuth endpoints",
		},
		cli.StringFlag{
			Name:  "custom-tenant-id",
			Value: "",
			Usage: "Use custom Tenant ID for OAuth endpoints",
		},
		cli.StringFlag{
			Name:  "custom-auth-url",
			Value: "",
			Usage: "Use a custom Authorization URL (option for GitLab/GitHub)",
		},
		cli.StringFlag{
			Name:  "custom-token-url",
			Value: "",
			Usage: "Use a custom Token URL (option for GitLab/GitHub)",
		},
		cli.StringFlag{
			Name:  "custom-profile-url",
			Value: "",
			Usage: "Use a custom Profile URL (option for GitLab/GitHub)",
		},
		cli.StringFlag{
			Name:  "custom-email-url",
			Value: "",
			Usage: "Use a custom Email URL (option for GitHub)",
		},
		cli.StringFlag{
			Name:  "icon-url",
			Value: "",
			Usage: "Custom icon URL for OAuth2 login source",
		},
		cli.BoolFlag{
			Name:  "skip-local-2fa",
			Usage: "Set to true to skip local 2fa for users authenticated by this source",
		},
		cli.StringSliceFlag{
			Name:  "scopes",
			Value: nil,
			Usage: "Scopes to request when to authenticate against this OAuth2 source",
		},
		cli.StringFlag{
			Name:  "required-claim-name",
			Value: "",
			Usage: "Claim name that has to be set to allow users to login with this source",
		},
		cli.StringFlag{
			Name:  "required-claim-value",
			Value: "",
			Usage: "Claim value that has to be set to allow users to login with this source",
		},
		cli.StringFlag{
			Name:  "group-claim-name",
			Value: "",
			Usage: "Claim name providing group names for this source",
		},
		cli.StringFlag{
			Name:  "admin-group",
			Value: "",
			Usage: "Group Claim value for administrator users",
		},
		cli.StringFlag{
			Name:  "restricted-group",
			Value: "",
			Usage: "Group Claim value for restricted users",
		},
		cli.StringFlag{
			Name:  "group-team-map",
			Value: "",
			Usage: "JSON mapping between groups and org teams",
		},
		cli.BoolFlag{
			Name:  "group-team-map-removal",
			Usage: "Activate automatic team membership removal depending on groups",
		},
	}

	microcmdAuthUpdateOauth = cli.Command{
		Name:   "update-oauth",
		Usage:  "Update existing Oauth authentication source",
		Action: runUpdateOauth,
		Flags:  append(oauthCLIFlags[:1], append([]cli.Flag{idFlag}, oauthCLIFlags[1:]...)...),
	}

	microcmdAuthAddOauth = cli.Command{
		Name:   "add-oauth",
		Usage:  "Add new Oauth authentication source",
		Action: runAddOauth,
		Flags:  oauthCLIFlags,
	}

	subcmdSendMail = cli.Command{
		Name:   "sendmail",
		Usage:  "Send a message to all users",
		Action: runSendMail,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "title",
				Usage: `a title of a message`,
				Value: "",
			},
			cli.StringFlag{
				Name:  "content",
				Usage: "a content of a message",
				Value: "",
			},
			cli.BoolFlag{
				Name:  "force,f",
				Usage: "A flag to bypass a confirmation step",
			},
		},
	}

	smtpCLIFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Value: "",
			Usage: "Application Name",
		},
		cli.StringFlag{
			Name:  "auth-type",
			Value: "PLAIN",
			Usage: "SMTP Authentication Type (PLAIN/LOGIN/CRAM-MD5) default PLAIN",
		},
		cli.StringFlag{
			Name:  "host",
			Value: "",
			Usage: "SMTP Host",
		},
		cli.IntFlag{
			Name:  "port",
			Usage: "SMTP Port",
		},
		cli.BoolTFlag{
			Name:  "force-smtps",
			Usage: "SMTPS is always used on port 465. Set this to force SMTPS on other ports.",
		},
		cli.BoolTFlag{
			Name:  "skip-verify",
			Usage: "Skip TLS verify.",
		},
		cli.StringFlag{
			Name:  "helo-hostname",
			Value: "",
			Usage: "Hostname sent with HELO. Leave blank to send current hostname",
		},
		cli.BoolTFlag{
			Name:  "disable-helo",
			Usage: "Disable SMTP helo.",
		},
		cli.StringFlag{
			Name:  "allowed-domains",
			Value: "",
			Usage: "Leave empty to allow all domains. Separate multiple domains with a comma (',')",
		},
		cli.BoolTFlag{
			Name:  "skip-local-2fa",
			Usage: "Skip 2FA to log on.",
		},
		cli.BoolTFlag{
			Name:  "active",
			Usage: "This Authentication Source is Activated.",
		},
	}

	microcmdAuthAddSMTP = cli.Command{
		Name:   "add-smtp",
		Usage:  "Add new SMTP authentication source",
		Action: runAddSMTP,
		Flags:  smtpCLIFlags,
	}

	microcmdAuthUpdateSMTP = cli.Command{
		Name:   "update-smtp",
		Usage:  "Update existing SMTP authentication source",
		Action: runUpdateSMTP,
		Flags:  append(smtpCLIFlags[:1], append([]cli.Flag{idFlag}, smtpCLIFlags[1:]...)...),
	}
)

func runRepoSyncReleases(_ *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	if err := git.InitSimple(ctx); err != nil {
		return err
	}

	log.Trace("Synchronizing repository releases (this may take a while)")
	for page := 1; ; page++ {
		repos, count, err := repo_model.SearchRepositoryByName(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: repo_model.RepositoryListDefaultPageSize,
				Page:     page,
			},
			Private: true,
		})
		if err != nil {
			return fmt.Errorf("SearchRepositoryByName: %w", err)
		}
		if len(repos) == 0 {
			break
		}
		log.Trace("Processing next %d repos of %d", len(repos), count)
		for _, repo := range repos {
			log.Trace("Synchronizing repo %s with path %s", repo.FullName(), repo.RepoPath())
			gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
			if err != nil {
				log.Warn("OpenRepository: %v", err)
				continue
			}

			oldnum, err := getReleaseCount(repo.ID)
			if err != nil {
				log.Warn(" GetReleaseCountByRepoID: %v", err)
			}
			log.Trace(" currentNumReleases is %d, running SyncReleasesWithTags", oldnum)

			if err = repo_module.SyncReleasesWithTags(repo, gitRepo); err != nil {
				log.Warn(" SyncReleasesWithTags: %v", err)
				gitRepo.Close()
				continue
			}

			count, err = getReleaseCount(repo.ID)
			if err != nil {
				log.Warn(" GetReleaseCountByRepoID: %v", err)
				gitRepo.Close()
				continue
			}

			log.Trace(" repo %s releases synchronized to tags: from %d to %d",
				repo.FullName(), oldnum, count)
			gitRepo.Close()
		}
	}

	return nil
}

func getReleaseCount(id int64) (int64, error) {
	return repo_model.GetReleaseCountByRepoID(
		db.DefaultContext,
		id,
		repo_model.FindReleasesOptions{
			IncludeTags: true,
		},
	)
}

func runRegenerateHooks(_ *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}
	return repo_service.SyncRepositoryHooks(graceful.GetManager().ShutdownContext())
}

func runRegenerateKeys(_ *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}
	return asymkey_model.RewriteAllPublicKeys()
}

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

	return auth_model.CreateSource(&auth_model.Source{
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

	source, err := auth_model.GetSourceByID(c.Int64("id"))
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

	return auth_model.UpdateSource(source)
}

func parseSMTPConfig(c *cli.Context, conf *smtp.Source) error {
	if c.IsSet("auth-type") {
		conf.Auth = c.String("auth-type")
		validAuthTypes := []string{"PLAIN", "LOGIN", "CRAM-MD5"}
		if !util.SliceContainsString(validAuthTypes, strings.ToUpper(c.String("auth-type"))) {
			return errors.New("Auth must be one of PLAIN/LOGIN/CRAM-MD5")
		}
		conf.Auth = c.String("auth-type")
	}
	if c.IsSet("host") {
		conf.Host = c.String("host")
	}
	if c.IsSet("port") {
		conf.Port = c.Int("port")
	}
	if c.IsSet("allowed-domains") {
		conf.AllowedDomains = c.String("allowed-domains")
	}
	if c.IsSet("force-smtps") {
		conf.ForceSMTPS = c.BoolT("force-smtps")
	}
	if c.IsSet("skip-verify") {
		conf.SkipVerify = c.BoolT("skip-verify")
	}
	if c.IsSet("helo-hostname") {
		conf.HeloHostname = c.String("helo-hostname")
	}
	if c.IsSet("disable-helo") {
		conf.DisableHelo = c.BoolT("disable-helo")
	}
	if c.IsSet("skip-local-2fa") {
		conf.SkipLocalTwoFA = c.BoolT("skip-local-2fa")
	}
	return nil
}

func runAddSMTP(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	if !c.IsSet("name") || len(c.String("name")) == 0 {
		return errors.New("name must be set")
	}
	if !c.IsSet("host") || len(c.String("host")) == 0 {
		return errors.New("host must be set")
	}
	if !c.IsSet("port") {
		return errors.New("port must be set")
	}
	active := true
	if c.IsSet("active") {
		active = c.BoolT("active")
	}

	var smtpConfig smtp.Source
	if err := parseSMTPConfig(c, &smtpConfig); err != nil {
		return err
	}

	// If not set default to PLAIN
	if len(smtpConfig.Auth) == 0 {
		smtpConfig.Auth = "PLAIN"
	}

	return auth_model.CreateSource(&auth_model.Source{
		Type:     auth_model.SMTP,
		Name:     c.String("name"),
		IsActive: active,
		Cfg:      &smtpConfig,
	})
}

func runUpdateSMTP(c *cli.Context) error {
	if !c.IsSet("id") {
		return fmt.Errorf("--id flag is missing")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	source, err := auth_model.GetSourceByID(c.Int64("id"))
	if err != nil {
		return err
	}

	smtpConfig := source.Cfg.(*smtp.Source)

	if err := parseSMTPConfig(c, smtpConfig); err != nil {
		return err
	}

	if c.IsSet("name") {
		source.Name = c.String("name")
	}

	if c.IsSet("active") {
		source.IsActive = c.BoolT("active")
	}

	source.Cfg = smtpConfig

	return auth_model.UpdateSource(source)
}

func runListAuth(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	authSources, err := auth_model.Sources()
	if err != nil {
		return err
	}

	flags := tabwriter.AlignRight
	if c.Bool("vertical-bars") {
		flags |= tabwriter.Debug
	}

	padChar := byte('\t')
	if len(c.String("pad-char")) > 0 {
		padChar = c.String("pad-char")[0]
	}

	// loop through each source and print
	w := tabwriter.NewWriter(os.Stdout, c.Int("min-width"), c.Int("tab-width"), c.Int("padding"), padChar, flags)
	fmt.Fprintf(w, "ID\tName\tType\tEnabled\n")
	for _, source := range authSources {
		fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", source.ID, source.Name, source.Type.String(), source.IsActive)
	}
	w.Flush()

	return nil
}

func runDeleteAuth(c *cli.Context) error {
	if !c.IsSet("id") {
		return fmt.Errorf("--id flag is missing")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	source, err := auth_model.GetSourceByID(c.Int64("id"))
	if err != nil {
		return err
	}

	return auth_service.DeleteSource(source)
}

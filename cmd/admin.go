// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	pwd "code.gitea.io/gitea/modules/password"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/auth/source/smtp"
	repo_service "code.gitea.io/gitea/services/repository"
	user_service "code.gitea.io/gitea/services/user"

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

	subcmdUser = cli.Command{
		Name:  "user",
		Usage: "Modify users",
		Subcommands: []cli.Command{
			microcmdUserCreate,
			microcmdUserList,
			microcmdUserChangePassword,
			microcmdUserDelete,
			microcmdUserGenerateAccessToken,
		},
	}

	microcmdUserList = cli.Command{
		Name:   "list",
		Usage:  "List users",
		Action: runListUsers,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "admin",
				Usage: "List only admin users",
			},
		},
	}

	microcmdUserCreate = cli.Command{
		Name:   "create",
		Usage:  "Create a new user in database",
		Action: runCreateUser,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name",
				Usage: "Username. DEPRECATED: use username instead",
			},
			cli.StringFlag{
				Name:  "username",
				Usage: "Username",
			},
			cli.StringFlag{
				Name:  "password",
				Usage: "User password",
			},
			cli.StringFlag{
				Name:  "email",
				Usage: "User email address",
			},
			cli.BoolFlag{
				Name:  "admin",
				Usage: "User is an admin",
			},
			cli.BoolFlag{
				Name:  "random-password",
				Usage: "Generate a random password for the user",
			},
			cli.BoolFlag{
				Name:  "must-change-password",
				Usage: "Set this option to false to prevent forcing the user to change their password after initial login, (Default: true)",
			},
			cli.IntFlag{
				Name:  "random-password-length",
				Usage: "Length of the random password to be generated",
				Value: 12,
			},
			cli.BoolFlag{
				Name:  "access-token",
				Usage: "Generate access token for the user",
			},
			cli.BoolFlag{
				Name:  "restricted",
				Usage: "Make a restricted user account",
			},
		},
	}

	microcmdUserChangePassword = cli.Command{
		Name:   "change-password",
		Usage:  "Change a user's password",
		Action: runChangePassword,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "username,u",
				Value: "",
				Usage: "The user to change password for",
			},
			cli.StringFlag{
				Name:  "password,p",
				Value: "",
				Usage: "New password to set for user",
			},
		},
	}

	microcmdUserDelete = cli.Command{
		Name:  "delete",
		Usage: "Delete specific user by id, name or email",
		Flags: []cli.Flag{
			cli.Int64Flag{
				Name:  "id",
				Usage: "ID of user of the user to delete",
			},
			cli.StringFlag{
				Name:  "username,u",
				Usage: "Username of the user to delete",
			},
			cli.StringFlag{
				Name:  "email,e",
				Usage: "Email of the user to delete",
			},
		},
		Action: runDeleteUser,
	}

	microcmdUserGenerateAccessToken = cli.Command{
		Name:  "generate-access-token",
		Usage: "Generate a access token for a specific user",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "username,u",
				Usage: "Username",
			},
			cli.StringFlag{
				Name:  "token-name,t",
				Usage: "Token name",
				Value: "gitea-admin",
			},
			cli.BoolFlag{
				Name:  "raw",
				Usage: "Display only the token value",
			},
		},
		Action: runGenerateAccessToken,
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

func runChangePassword(c *cli.Context) error {
	if err := argsSet(c, "username", "password"); err != nil {
		return err
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}
	if len(c.String("password")) < setting.MinPasswordLength {
		return fmt.Errorf("Password is not long enough. Needs to be at least %d", setting.MinPasswordLength)
	}

	if !pwd.IsComplexEnough(c.String("password")) {
		return errors.New("Password does not meet complexity requirements")
	}
	pwned, err := pwd.IsPwned(context.Background(), c.String("password"))
	if err != nil {
		return err
	}
	if pwned {
		return errors.New("The password you chose is on a list of stolen passwords previously exposed in public data breaches. Please try again with a different password.\nFor more details, see https://haveibeenpwned.com/Passwords")
	}
	uname := c.String("username")
	user, err := user_model.GetUserByName(ctx, uname)
	if err != nil {
		return err
	}
	if err = user.SetPassword(c.String("password")); err != nil {
		return err
	}

	if err = user_model.UpdateUserCols(ctx, user, "passwd", "passwd_hash_algo", "salt"); err != nil {
		return err
	}

	fmt.Printf("%s's password has been successfully updated!\n", user.Name)
	return nil
}

func runCreateUser(c *cli.Context) error {
	if err := argsSet(c, "email"); err != nil {
		return err
	}

	if c.IsSet("name") && c.IsSet("username") {
		return errors.New("Cannot set both --name and --username flags")
	}
	if !c.IsSet("name") && !c.IsSet("username") {
		return errors.New("One of --name or --username flags must be set")
	}

	if c.IsSet("password") && c.IsSet("random-password") {
		return errors.New("cannot set both -random-password and -password flags")
	}

	var username string
	if c.IsSet("username") {
		username = c.String("username")
	} else {
		username = c.String("name")
		fmt.Fprintf(os.Stderr, "--name flag is deprecated. Use --username instead.\n")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	var password string
	if c.IsSet("password") {
		password = c.String("password")
	} else if c.IsSet("random-password") {
		var err error
		password, err = pwd.Generate(c.Int("random-password-length"))
		if err != nil {
			return err
		}
		fmt.Printf("generated random password is '%s'\n", password)
	} else {
		return errors.New("must set either password or random-password flag")
	}

	// always default to true
	changePassword := true

	// If this is the first user being created.
	// Take it as the admin and don't force a password update.
	if n := user_model.CountUsers(nil); n == 0 {
		changePassword = false
	}

	if c.IsSet("must-change-password") {
		changePassword = c.Bool("must-change-password")
	}

	restricted := util.OptionalBoolNone

	if c.IsSet("restricted") {
		restricted = util.OptionalBoolOf(c.Bool("restricted"))
	}

	u := &user_model.User{
		Name:               username,
		Email:              c.String("email"),
		Passwd:             password,
		IsAdmin:            c.Bool("admin"),
		MustChangePassword: changePassword,
	}

	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive:     util.OptionalBoolTrue,
		IsRestricted: restricted,
	}

	if err := user_model.CreateUser(u, overwriteDefault); err != nil {
		return fmt.Errorf("CreateUser: %v", err)
	}

	if c.Bool("access-token") {
		t := &models.AccessToken{
			Name: "gitea-admin",
			UID:  u.ID,
		}

		if err := models.NewAccessToken(t); err != nil {
			return err
		}

		fmt.Printf("Access token was successfully created... %s\n", t.Token)
	}

	fmt.Printf("New user '%s' has been successfully created!\n", username)
	return nil
}

func runListUsers(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	users, err := user_model.GetAllUsers()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 0, 1, ' ', 0)

	if c.IsSet("admin") {
		fmt.Fprintf(w, "ID\tUsername\tEmail\tIsActive\n")
		for _, u := range users {
			if u.IsAdmin {
				fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", u.ID, u.Name, u.Email, u.IsActive)
			}
		}
	} else {
		fmt.Fprintf(w, "ID\tUsername\tEmail\tIsActive\tIsAdmin\n")
		for _, u := range users {
			fmt.Fprintf(w, "%d\t%s\t%s\t%t\t%t\n", u.ID, u.Name, u.Email, u.IsActive, u.IsAdmin)
		}

	}

	w.Flush()
	return nil
}

func runDeleteUser(c *cli.Context) error {
	if !c.IsSet("id") && !c.IsSet("username") && !c.IsSet("email") {
		return fmt.Errorf("You must provide the id, username or email of a user to delete")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	var err error
	var user *user_model.User
	if c.IsSet("email") {
		user, err = user_model.GetUserByEmail(c.String("email"))
	} else if c.IsSet("username") {
		user, err = user_model.GetUserByName(ctx, c.String("username"))
	} else {
		user, err = user_model.GetUserByID(c.Int64("id"))
	}
	if err != nil {
		return err
	}
	if c.IsSet("username") && user.LowerName != strings.ToLower(strings.TrimSpace(c.String("username"))) {
		return fmt.Errorf("The user %s who has email %s does not match the provided username %s", user.Name, c.String("email"), c.String("username"))
	}

	if c.IsSet("id") && user.ID != c.Int64("id") {
		return fmt.Errorf("The user %s does not match the provided id %d", user.Name, c.Int64("id"))
	}

	return user_service.DeleteUser(user)
}

func runGenerateAccessToken(c *cli.Context) error {
	if !c.IsSet("username") {
		return fmt.Errorf("You must provide the username to generate a token for them")
	}

	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	user, err := user_model.GetUserByName(ctx, c.String("username"))
	if err != nil {
		return err
	}

	t := &models.AccessToken{
		Name: c.String("token-name"),
		UID:  user.ID,
	}

	if err := models.NewAccessToken(t); err != nil {
		return err
	}

	if c.Bool("raw") {
		fmt.Printf("%s\n", t.Token)
	} else {
		fmt.Printf("Access token was successfully created: %s\n", t.Token)
	}

	return nil
}

func runRepoSyncReleases(_ *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	log.Trace("Synchronizing repository releases (this may take a while)")
	for page := 1; ; page++ {
		repos, count, err := models.SearchRepositoryByName(&models.SearchRepoOptions{
			ListOptions: db.ListOptions{
				PageSize: models.RepositoryListDefaultPageSize,
				Page:     page,
			},
			Private: true,
		})
		if err != nil {
			return fmt.Errorf("SearchRepositoryByName: %v", err)
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
	return models.GetReleaseCountByRepoID(
		id,
		models.FindReleasesOptions{
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
	}
}

func runAddOauth(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	return auth.CreateSource(&auth.Source{
		Type:     auth.OAuth2,
		Name:     c.String("name"),
		IsActive: true,
		Cfg:      parseOAuth2Config(c),
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

	source, err := auth.GetSourceByID(c.Int64("id"))
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

	// update custom URL mapping
	customURLMapping := &oauth2.CustomURLMapping{}

	if oAuth2Config.CustomURLMapping != nil {
		customURLMapping.TokenURL = oAuth2Config.CustomURLMapping.TokenURL
		customURLMapping.AuthURL = oAuth2Config.CustomURLMapping.AuthURL
		customURLMapping.ProfileURL = oAuth2Config.CustomURLMapping.ProfileURL
		customURLMapping.EmailURL = oAuth2Config.CustomURLMapping.EmailURL
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

	oAuth2Config.CustomURLMapping = customURLMapping
	source.Cfg = oAuth2Config

	return auth.UpdateSource(source)
}

func parseSMTPConfig(c *cli.Context, conf *smtp.Source) error {
	if c.IsSet("auth-type") {
		conf.Auth = c.String("auth-type")
		validAuthTypes := []string{"PLAIN", "LOGIN", "CRAM-MD5"}
		if !contains(validAuthTypes, strings.ToUpper(c.String("auth-type"))) {
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

	return auth.CreateSource(&auth.Source{
		Type:     auth.SMTP,
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

	source, err := auth.GetSourceByID(c.Int64("id"))
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

	return auth.UpdateSource(source)
}

func runListAuth(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	authSources, err := auth.Sources()
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

	source, err := auth.GetSourceByID(c.Int64("id"))
	if err != nil {
		return err
	}

	return auth_service.DeleteSource(source)
}

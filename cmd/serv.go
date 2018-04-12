// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
	"github.com/dgrijalva/jwt-go"
	"github.com/urfave/cli"
)

const (
	accessDenied        = "Repository does not exist or you do not have access"
	lfsAuthenticateVerb = "git-lfs-authenticate"
	gitAnnexVerb        = "git-annex-shell"
)

// CmdServ represents the available serv sub-command.
var CmdServ = cli.Command{
	Name:        "serv",
	Usage:       "This command should only be called by SSH shell",
	Description: `Serv provide access auth for repositories`,
	Action:      runServ,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "custom/conf/app.ini",
			Usage: "Custom configuration file path",
		},
	},
}

func setup(logPath string) error {
	setting.NewContext()
	log.NewGitLogger(filepath.Join(setting.LogRootPath, logPath))
	models.LoadConfigs()

	if setting.UseSQLite3 || setting.UseTiDB {
		workPath := setting.AppWorkPath
		if err := os.Chdir(workPath); err != nil {
			log.GitLogger.Fatal(4, "Failed to change directory %s: %v", workPath, err)
		}
	}

	setting.NewXORMLogService(true)
	return models.SetEngine()
}

func parseCmd(cmd string) (string, []string) {
	parts := strings.Split(cmd, " ")
	for i := range parts {
		parts[i] = strings.Trim(parts[i], "'")
	}
	return parts[0], parts[1:]
}

var (
	allowedCommands = map[string]models.AccessMode{
		"git-upload-pack":    models.AccessModeRead,
		"git-upload-archive": models.AccessModeRead,
		"git-receive-pack":   models.AccessModeWrite,
		lfsAuthenticateVerb:  models.AccessModeNone,
		gitAnnexVerb:         models.AccessModeNone,
	}
	gitAnnexCommands = map[string]models.AccessMode{
		"commit":     models.AccessModeWrite,
		"configlist": models.AccessModeRead,
		"dropkey":    models.AccessModeWrite,
		"inannex":    models.AccessModeRead,
		"recvkey":    models.AccessModeWrite,
		"sendkey":    models.AccessModeRead,
	}
	lfsAuthenticateCommands = map[string]models.AccessMode{
		"upload":   models.AccessModeWrite,
		"download": models.AccessModeRead,
	}
)

func fail(userMessage, logMessage string, args ...interface{}) {
	fmt.Fprintln(os.Stderr, "Gitea:", userMessage)

	if len(logMessage) > 0 {
		if !setting.ProdMode {
			fmt.Fprintf(os.Stderr, logMessage+"\n", args...)
		}
		log.GitLogger.Fatal(3, logMessage, args...)
		return
	}

	log.GitLogger.Close()
	os.Exit(1)
}

func runServ(c *cli.Context) error {
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if err := setup("serv.log"); err != nil {
		fail("System init failed", fmt.Sprintf("setup: %v", err))
	}

	if setting.SSH.Disabled {
		println("Gitea: SSH has been disabled")
		return nil
	}

	if len(c.Args()) < 1 {
		cli.ShowSubcommandHelp(c)
		return nil
	}

	cmd := os.Getenv("SSH_ORIGINAL_COMMAND")
	if len(cmd) == 0 {
		println("Hi there, You've successfully authenticated, but Gitea does not provide shell access.")
		println("If this is unexpected, please log in with password and setup Gitea under another user.")
		return nil
	}

	verb, args := parseCmd(cmd)
	repoPath := args[0]
	expected := 1
	requestedMode, allowed := allowedCommands[verb]

	if !allowed {
		fail("Unknown git command", "Unknown command %s", verb)
	}

	switch verb {
	case gitAnnexVerb:
		if !setting.GitAnnex.Enabled {
			fail("Unknown git command", "Git-Annex support is disabled")
		}
		expected = -1
		repoPath = strings.Replace(args[1], "/~/", "", 1)
		args[1] = strings.Replace(args[1], "/~", setting.RepoRootPath, 1)
		requestedMode, allowed = gitAnnexCommands[args[0]]
	case lfsAuthenticateVerb:
		if !setting.LFS.StartServer {
			fail("Unknown git command", "LFS authentication request over SSH denied, LFS support is disabled")
		}
		expected = 2
		requestedMode, allowed = lfsAuthenticateCommands[args[1]]
	}

	// check we havent been pass extra args
	if expected > 0 && len(args) != expected {
		fail("Unknown git command", "Expected %d arguments for %s", expected, verb)
	}

	if !allowed {
		fail("Unknown git command", "Unknown subcommand for %s: %s", verb, args[0])
	}

	repoPath = strings.ToLower(repoPath)
	rr := strings.SplitN(repoPath, "/", 2)
	if len(rr) != 2 {
		fail("Invalid repository path", "Invalid repository path: %v", repoPath)
	}

	username := strings.ToLower(rr[0])
	reponame := strings.ToLower(strings.TrimSuffix(rr[1], ".git"))

	isWiki := false
	unitType := models.UnitTypeCode
	if strings.HasSuffix(reponame, ".wiki") {
		isWiki = true
		unitType = models.UnitTypeWiki
		reponame = reponame[:len(reponame)-5]
	}

	os.Setenv(models.EnvRepoUsername, username)
	if isWiki {
		os.Setenv(models.EnvRepoIsWiki, "true")
	} else {
		os.Setenv(models.EnvRepoIsWiki, "false")
	}
	os.Setenv(models.EnvRepoName, reponame)

	repo, err := models.GetRepositoryByOwnerAndName(username, reponame)
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			fail(accessDenied, "Repository does not exist: %s/%s", username, reponame)
		}
		fail("Internal error", "Failed to get repository: %v", err)
	}

	// Prohibit push to mirror repositories.
	if requestedMode > models.AccessModeRead && repo.IsMirror {
		fail("mirror repository is read-only", "")
	}

	// Allow anonymous clone for public repositories.
	var (
		keyID int64
		user  *models.User
	)
	if requestedMode == models.AccessModeWrite || repo.IsPrivate {
		keys := strings.Split(c.Args()[0], "-")
		if len(keys) != 2 {
			fail("Key ID format error", "Invalid key argument: %s", c.Args()[0])
		}

		key, err := models.GetPublicKeyByID(com.StrTo(keys[1]).MustInt64())
		if err != nil {
			fail("Invalid key ID", "Invalid key ID[%s]: %v", c.Args()[0], err)
		}
		keyID = key.ID

		// Check deploy key or user key.
		if key.Type == models.KeyTypeDeploy {
			if key.Mode < requestedMode {
				fail("Key permission denied", "Cannot push with deployment key: %d", key.ID)
			}
			// Check if this deploy key belongs to current repository.
			if !models.HasDeployKey(key.ID, repo.ID) {
				fail("Key access denied", "Deploy key access denied: [key_id: %d, repo_id: %d]", key.ID, repo.ID)
			}

			// Update deploy key activity.
			deployKey, err := models.GetDeployKeyByRepo(key.ID, repo.ID)
			if err != nil {
				fail("Internal error", "GetDeployKey: %v", err)
			}

			deployKey.UpdatedUnix = util.TimeStampNow()
			if err = models.UpdateDeployKeyCols(deployKey, "updated_unix"); err != nil {
				fail("Internal error", "UpdateDeployKey: %v", err)
			}
		} else {
			user, err = models.GetUserByKeyID(key.ID)
			if err != nil {
				fail("internal error", "Failed to get user by key ID(%d): %v", keyID, err)
			}

			mode, err := models.AccessLevel(user.ID, repo)
			if err != nil {
				fail("Internal error", "Failed to check access: %v", err)
			} else if mode < requestedMode {
				clientMessage := accessDenied
				if mode >= models.AccessModeRead {
					clientMessage = "You do not have sufficient authorization for this action"
				}
				fail(clientMessage,
					"User %s does not have level %v access to repository %s",
					user.Name, requestedMode, repoPath)
			}

			if !repo.CheckUnitUser(user.ID, user.IsAdmin, unitType) {
				fail("You do not have allowed for this action",
					"User %s does not have allowed access to repository %s 's code",
					user.Name, repoPath)
			}

			os.Setenv(models.EnvPusherName, user.Name)
			os.Setenv(models.EnvPusherID, fmt.Sprintf("%d", user.ID))
		}
	}

	//LFS token authentication
	if verb == lfsAuthenticateVerb {
		url := fmt.Sprintf("%s%s/%s.git/info/lfs", setting.AppURL, username, repo.Name)

		now := time.Now()
		claims := jwt.MapClaims{
			"repo": repo.ID,
			"op":   args[1],
			"exp":  now.Add(5 * time.Minute).Unix(),
			"nbf":  now.Unix(),
		}
		if user != nil {
			claims["user"] = user.ID
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString(setting.LFS.JWTSecretBytes)
		if err != nil {
			fail("Internal error", "Failed to sign JWT token: %v", err)
		}

		tokenAuthentication := &models.LFSTokenResponse{
			Header: make(map[string]string),
			Href:   url,
		}
		tokenAuthentication.Header["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)

		enc := json.NewEncoder(os.Stdout)
		err = enc.Encode(tokenAuthentication)
		if err != nil {
			fail("Internal error", "Failed to encode LFS json response: %v", err)
		}

		return nil
	}

	// Special handle for Windows.
	if setting.IsWindows {
		parts := strings.SplitN(verb, "-", 2)
		verb = parts[0]
		args = append(parts[1:], args...)
	}

	var gitcmd *exec.Cmd
	log.GitLogger.Info("Executing: %v %v", verb, args)
	gitcmd = exec.Command(verb, args...)

	if isWiki {
		if err = repo.InitWiki(); err != nil {
			fail("Internal error", "Failed to init wiki repo: %v", err)
		}
	}

	os.Setenv(models.ProtectedBranchRepoID, fmt.Sprintf("%d", repo.ID))

	gitcmd.Dir = setting.RepoRootPath
	gitcmd.Stdout = os.Stdout
	gitcmd.Stdin = os.Stdin
	gitcmd.Stderr = os.Stderr
	if err = gitcmd.Run(); err != nil {
		if args[0] == "inannex" {
			fail("Not present", "")
		}
		fail("Internal error", "Failed to execute git command: %v", err)
	}

	// Update user key activity.
	if keyID > 0 {
		if err = private.UpdatePublicKeyUpdated(keyID); err != nil {
			fail("Internal error", "UpdatePublicKey: %v", err)
		}
	}

	return nil
}

// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/pprof"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/process"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/lfs"

	"github.com/golang-jwt/jwt/v4"
	"github.com/kballard/go-shellquote"
	"github.com/urfave/cli"
)

const (
	lfsAuthenticateVerb = "git-lfs-authenticate"
)

// CmdServ represents the available serv sub-command.
var CmdServ = cli.Command{
	Name:        "serv",
	Usage:       "This command should only be called by SSH shell",
	Description: `Serv provide access auth for repositories`,
	Action:      runServ,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name: "enable-pprof",
		},
		cli.BoolFlag{
			Name: "debug",
		},
	},
}

func setup(logPath string, debug bool) {
	_ = log.DelLogger("console")
	if debug {
		_ = log.NewLogger(1000, "console", "console", `{"level":"trace","stacktracelevel":"NONE","stderr":true}`)
	} else {
		_ = log.NewLogger(1000, "console", "console", `{"level":"fatal","stacktracelevel":"NONE","stderr":true}`)
	}
	setting.LoadFromExisting()
	if debug {
		setting.RunMode = "dev"
	}

	// Check if setting.RepoRootPath exists. It could be the case that it doesn't exist, this can happen when
	// `[repository]` `ROOT` is a relative path and $GITEA_WORK_DIR isn't passed to the SSH connection.
	if _, err := os.Stat(setting.RepoRootPath); err != nil {
		if os.IsNotExist(err) {
			_ = fail("Incorrect configuration, no repository directory.", "Directory `[repository].ROOT` %q was not found, please check if $GITEA_WORK_DIR is passed to the SSH connection or make `[repository].ROOT` an absolute value.", setting.RepoRootPath)
		} else {
			_ = fail("Incorrect configuration, repository directory is inaccessible", "Directory `[repository].ROOT` %q is inaccessible. err: %v", setting.RepoRootPath, err)
		}
		return
	}

	if err := git.InitSimple(context.Background()); err != nil {
		_ = fail("Failed to init git", "Failed to init git, err: %v", err)
	}
}

var (
	allowedCommands = map[string]perm.AccessMode{
		"git-upload-pack":    perm.AccessModeRead,
		"git-upload-archive": perm.AccessModeRead,
		"git-receive-pack":   perm.AccessModeWrite,
		lfsAuthenticateVerb:  perm.AccessModeNone,
	}
	alphaDashDotPattern = regexp.MustCompile(`[^\w-\.]`)
)

func fail(userMessage, logMessage string, args ...interface{}) error {
	// There appears to be a chance to cause a zombie process and failure to read the Exit status
	// if nothing is outputted on stdout.
	_, _ = fmt.Fprintln(os.Stdout, "")
	_, _ = fmt.Fprintln(os.Stderr, "Gitea:", userMessage)

	if len(logMessage) > 0 {
		if !setting.IsProd {
			_, _ = fmt.Fprintf(os.Stderr, logMessage+"\n", args...)
		}
	}
	ctx, cancel := installSignals()
	defer cancel()

	if len(logMessage) > 0 {
		_ = private.SSHLog(ctx, true, fmt.Sprintf(logMessage+": ", args...))
	}
	return cli.NewExitError("", 1)
}

func runServ(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	// FIXME: This needs to internationalised
	setup("serv.log", c.Bool("debug"))

	if setting.SSH.Disabled {
		println("Gitea: SSH has been disabled")
		return nil
	}

	if len(c.Args()) < 1 {
		if err := cli.ShowSubcommandHelp(c); err != nil {
			fmt.Printf("error showing subcommand help: %v\n", err)
		}
		return nil
	}

	keys := strings.Split(c.Args()[0], "-")
	if len(keys) != 2 || keys[0] != "key" {
		return fail("Key ID format error", "Invalid key argument: %s", c.Args()[0])
	}
	keyID, err := strconv.ParseInt(keys[1], 10, 64)
	if err != nil {
		return fail("Key ID format error", "Invalid key argument: %s", c.Args()[1])
	}

	cmd := os.Getenv("SSH_ORIGINAL_COMMAND")
	if len(cmd) == 0 {
		key, user, err := private.ServNoCommand(ctx, keyID)
		if err != nil {
			return fail("Internal error", "Failed to check provided key: %v", err)
		}
		switch key.Type {
		case asymkey_model.KeyTypeDeploy:
			println("Hi there! You've successfully authenticated with the deploy key named " + key.Name + ", but Gitea does not provide shell access.")
		case asymkey_model.KeyTypePrincipal:
			println("Hi there! You've successfully authenticated with the principal " + key.Content + ", but Gitea does not provide shell access.")
		default:
			println("Hi there, " + user.Name + "! You've successfully authenticated with the key named " + key.Name + ", but Gitea does not provide shell access.")
		}
		println("If this is unexpected, please log in with password and setup Gitea under another user.")
		return nil
	} else if c.Bool("debug") {
		log.Debug("SSH_ORIGINAL_COMMAND: %s", os.Getenv("SSH_ORIGINAL_COMMAND"))
	}

	words, err := shellquote.Split(cmd)
	if err != nil {
		return fail("Error parsing arguments", "Failed to parse arguments: %v", err)
	}

	if len(words) < 2 {
		if git.CheckGitVersionAtLeast("2.29") == nil {
			// for AGit Flow
			if cmd == "ssh_info" {
				fmt.Print(`{"type":"gitea","version":1}`)
				return nil
			}
		}
		return fail("Too few arguments", "Too few arguments in cmd: %s", cmd)
	}

	verb := words[0]
	repoPath := words[1]
	if repoPath[0] == '/' {
		repoPath = repoPath[1:]
	}

	var lfsVerb string
	if verb == lfsAuthenticateVerb {
		if !setting.LFS.StartServer {
			return fail("Unknown git command", "LFS authentication request over SSH denied, LFS support is disabled")
		}

		if len(words) > 2 {
			lfsVerb = words[2]
		}
	}

	// LowerCase and trim the repoPath as that's how they are stored.
	repoPath = strings.ToLower(strings.TrimSpace(repoPath))

	rr := strings.SplitN(repoPath, "/", 2)
	if len(rr) != 2 {
		return fail("Invalid repository path", "Invalid repository path: %v", repoPath)
	}

	username := strings.ToLower(rr[0])
	reponame := strings.ToLower(strings.TrimSuffix(rr[1], ".git"))

	if alphaDashDotPattern.MatchString(reponame) {
		return fail("Invalid repo name", "Invalid repo name: %s", reponame)
	}

	if c.Bool("enable-pprof") {
		if err := os.MkdirAll(setting.PprofDataPath, os.ModePerm); err != nil {
			return fail("Error while trying to create PPROF_DATA_PATH", "Error while trying to create PPROF_DATA_PATH: %v", err)
		}

		stopCPUProfiler, err := pprof.DumpCPUProfileForUsername(setting.PprofDataPath, username)
		if err != nil {
			return fail("Internal Server Error", "Unable to start CPU profile: %v", err)
		}
		defer func() {
			stopCPUProfiler()
			err := pprof.DumpMemProfileForUsername(setting.PprofDataPath, username)
			if err != nil {
				_ = fail("Internal Server Error", "Unable to dump Mem Profile: %v", err)
			}
		}()
	}

	requestedMode, has := allowedCommands[verb]
	if !has {
		return fail("Unknown git command", "Unknown git command %s", verb)
	}

	if verb == lfsAuthenticateVerb {
		if lfsVerb == "upload" {
			requestedMode = perm.AccessModeWrite
		} else if lfsVerb == "download" {
			requestedMode = perm.AccessModeRead
		} else {
			return fail("Unknown LFS verb", "Unknown lfs verb %s", lfsVerb)
		}
	}

	results, err := private.ServCommand(ctx, keyID, username, reponame, requestedMode, verb, lfsVerb)
	if err != nil {
		if private.IsErrServCommand(err) {
			errServCommand := err.(private.ErrServCommand)
			if errServCommand.StatusCode != http.StatusInternalServerError {
				return fail("Unauthorized", "%s", errServCommand.Error())
			}
			return fail("Internal Server Error", "%s", errServCommand.Error())
		}
		return fail("Internal Server Error", "%s", err.Error())
	}

	// LFS token authentication
	if verb == lfsAuthenticateVerb {
		url := fmt.Sprintf("%s%s/%s.git/info/lfs", setting.AppURL, url.PathEscape(results.OwnerName), url.PathEscape(results.RepoName))

		now := time.Now()
		claims := lfs.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(now.Add(setting.LFS.HTTPAuthExpiry)),
				NotBefore: jwt.NewNumericDate(now),
			},
			RepoID: results.RepoID,
			Op:     lfsVerb,
			UserID: results.UserID,
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString(setting.LFS.JWTSecretBytes)
		if err != nil {
			return fail("Internal error", "Failed to sign JWT token: %v", err)
		}

		tokenAuthentication := &git_model.LFSTokenResponse{
			Header: make(map[string]string),
			Href:   url,
		}
		tokenAuthentication.Header["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)

		enc := json.NewEncoder(os.Stdout)
		err = enc.Encode(tokenAuthentication)
		if err != nil {
			return fail("Internal error", "Failed to encode LFS json response: %v", err)
		}
		return nil
	}

	// Special handle for Windows.
	if setting.IsWindows {
		verb = strings.Replace(verb, "-", " ", 1)
	}

	var gitcmd *exec.Cmd
	verbs := strings.Split(verb, " ")
	if len(verbs) == 2 {
		gitcmd = exec.CommandContext(ctx, verbs[0], verbs[1], repoPath)
	} else {
		gitcmd = exec.CommandContext(ctx, verb, repoPath)
	}

	process.SetSysProcAttribute(gitcmd)
	gitcmd.Dir = setting.RepoRootPath
	gitcmd.Stdout = os.Stdout
	gitcmd.Stdin = os.Stdin
	gitcmd.Stderr = os.Stderr
	gitcmd.Env = append(gitcmd.Env, os.Environ()...)
	gitcmd.Env = append(gitcmd.Env,
		repo_module.EnvRepoIsWiki+"="+strconv.FormatBool(results.IsWiki),
		repo_module.EnvRepoName+"="+results.RepoName,
		repo_module.EnvRepoUsername+"="+results.OwnerName,
		repo_module.EnvPusherName+"="+results.UserName,
		repo_module.EnvPusherEmail+"="+results.UserEmail,
		repo_module.EnvPusherID+"="+strconv.FormatInt(results.UserID, 10),
		repo_module.EnvRepoID+"="+strconv.FormatInt(results.RepoID, 10),
		repo_module.EnvPRID+"="+fmt.Sprintf("%d", 0),
		repo_module.EnvDeployKeyID+"="+fmt.Sprintf("%d", results.DeployKeyID),
		repo_module.EnvKeyID+"="+fmt.Sprintf("%d", results.KeyID),
		repo_module.EnvAppURL+"="+setting.AppURL,
	)
	// to avoid breaking, here only use the minimal environment variables for the "gitea serv" command.
	// it could be re-considered whether to use the same git.CommonGitCmdEnvs() as "git" command later.
	gitcmd.Env = append(gitcmd.Env, git.CommonCmdServEnvs()...)

	if err = gitcmd.Run(); err != nil {
		return fail("Internal error", "Failed to execute git command: %v", err)
	}

	// Update user key activity.
	if results.KeyID > 0 {
		if err = private.UpdatePublicKeyInRepo(ctx, results.KeyID, results.RepoID); err != nil {
			return fail("Internal error", "UpdatePublicKeyInRepo: %v", err)
		}
	}

	return nil
}

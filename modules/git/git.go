// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/hashicorp/go-version"
)

const RequiredVersion = "2.0.0" // the minimum Git version required

type Features struct {
	gitVersion *version.Version

	UsingGogit             bool
	SupportProcReceive     bool           // >= 2.29
	SupportHashSha256      bool           // >= 2.42, SHA-256 repositories no longer an ‘experimental curiosity’
	SupportedObjectFormats []ObjectFormat // sha1, sha256
}

var (
	GitExecutable   = "git"         // the command name of git, will be updated to an absolute path during initialization
	DefaultContext  context.Context // the default context to run git commands in, must be initialized by git.InitXxx
	defaultFeatures *Features
)

func (f *Features) CheckVersionAtLeast(atLeast string) bool {
	return f.gitVersion.Compare(version.Must(version.NewVersion(atLeast))) >= 0
}

// VersionInfo returns git version information
func (f *Features) VersionInfo() string {
	return f.gitVersion.Original()
}

func DefaultFeatures() *Features {
	if defaultFeatures == nil {
		if !setting.IsProd || setting.IsInTesting {
			log.Warn("git.DefaultFeatures is called before git.InitXxx, initializing with default values")
		}
		if err := InitSimple(context.Background()); err != nil {
			log.Fatal("git.InitSimple failed: %v", err)
		}
	}
	return defaultFeatures
}

func loadGitVersionFeatures() (*Features, error) {
	stdout, _, runErr := NewCommand(DefaultContext, "version").RunStdString(nil)
	if runErr != nil {
		return nil, runErr
	}

	ver, err := parseGitVersionLine(strings.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}

	features := &Features{gitVersion: ver, UsingGogit: isGogit}
	features.SupportProcReceive = features.CheckVersionAtLeast("2.29")
	features.SupportHashSha256 = features.CheckVersionAtLeast("2.42") && !isGogit
	features.SupportedObjectFormats = []ObjectFormat{Sha1ObjectFormat}
	if features.SupportHashSha256 {
		features.SupportedObjectFormats = append(features.SupportedObjectFormats, Sha256ObjectFormat)
	}
	return features, nil
}

func parseGitVersionLine(s string) (*version.Version, error) {
	fields := strings.Fields(s)
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid git version: %q", s)
	}

	// version string is like: "git version 2.29.3" or "git version 2.29.3.windows.1"
	versionString := fields[2]
	if pos := strings.Index(versionString, "windows"); pos >= 1 {
		versionString = versionString[:pos-1]
	}
	return version.NewVersion(versionString)
}

func checkGitVersionCompatibility(gitVer *version.Version) error {
	badVersions := []struct {
		Version *version.Version
		Reason  string
	}{
		{version.Must(version.NewVersion("2.43.1")), "regression bug of GIT_FLUSH"},
	}
	for _, bad := range badVersions {
		if gitVer.Equal(bad.Version) {
			return errors.New(bad.Reason)
		}
	}
	return nil
}

func ensureGitVersion() error {
	if !DefaultFeatures().CheckVersionAtLeast(RequiredVersion) {
		moreHint := "get git: https://git-scm.com/downloads"
		if runtime.GOOS == "linux" {
			// there are a lot of CentOS/RHEL users using old git, so we add a special hint for them
			if _, err := os.Stat("/etc/redhat-release"); err == nil {
				// ius.io is the recommended official(git-scm.com) method to install git
				moreHint = "get git: https://git-scm.com/downloads/linux and https://ius.io"
			}
		}
		return fmt.Errorf("installed git version %q is not supported, Gitea requires git version >= %q, %s", DefaultFeatures().gitVersion.Original(), RequiredVersion, moreHint)
	}

	if err := checkGitVersionCompatibility(DefaultFeatures().gitVersion); err != nil {
		return fmt.Errorf("installed git version %s has a known compatibility issue with Gitea: %w, please upgrade (or downgrade) git", DefaultFeatures().gitVersion.String(), err)
	}
	return nil
}

// SetExecutablePath changes the path of git executable and checks the file permission and version.
func SetExecutablePath(path string) error {
	// If path is empty, we use the default value of GitExecutable "git" to search for the location of git.
	if path != "" {
		GitExecutable = path
	}
	absPath, err := exec.LookPath(GitExecutable)
	if err != nil {
		return fmt.Errorf("git not found: %w", err)
	}
	GitExecutable = absPath
	return nil
}

// HomeDir is the home dir for git to store the global config file used by Gitea internally
func HomeDir() string {
	if setting.Git.HomePath == "" {
		// strict check, make sure the git module is initialized correctly.
		// attention: when the git module is called in gitea sub-command (serv/hook), the log module might not obviously show messages to users/developers.
		// for example: if there is gitea git hook code calling git.NewCommand before git.InitXxx, the integration test won't show the real failure reasons.
		log.Fatal("Unable to init Git's HomeDir, incorrect initialization of the setting and git modules")
		return ""
	}
	return setting.Git.HomePath
}

// InitSimple initializes git module with a very simple step, no config changes, no global command arguments.
// This method doesn't change anything to filesystem. At the moment, it is only used by some Gitea sub-commands.
func InitSimple(ctx context.Context) error {
	if setting.Git.HomePath == "" {
		return errors.New("unable to init Git's HomeDir, incorrect initialization of the setting and git modules")
	}

	if DefaultContext != nil && (!setting.IsProd || setting.IsInTesting) {
		log.Warn("git module has been initialized already, duplicate init may work but it's better to fix it")
	}

	DefaultContext = ctx
	globalCommandArgs = nil

	if setting.Git.Timeout.Default > 0 {
		defaultCommandExecutionTimeout = time.Duration(setting.Git.Timeout.Default) * time.Second
	}

	if err := SetExecutablePath(setting.Git.Path); err != nil {
		return err
	}

	var err error
	defaultFeatures, err = loadGitVersionFeatures()
	if err != nil {
		return err
	}
	if err = ensureGitVersion(); err != nil {
		return err
	}

	// when git works with gnupg (commit signing), there should be a stable home for gnupg commands
	if _, ok := os.LookupEnv("GNUPGHOME"); !ok {
		_ = os.Setenv("GNUPGHOME", filepath.Join(HomeDir(), ".gnupg"))
	}
	return nil
}

// InitFull initializes git module with version check and change global variables, sync gitconfig.
// It should only be called once at the beginning of the program initialization (TestMain/GlobalInitInstalled) as this code makes unsynchronized changes to variables.
func InitFull(ctx context.Context) (err error) {
	if err = InitSimple(ctx); err != nil {
		return err
	}

	// Since git wire protocol has been released from git v2.18
	if setting.Git.EnableAutoGitWireProtocol && DefaultFeatures().CheckVersionAtLeast("2.18") {
		globalCommandArgs = append(globalCommandArgs, "-c", "protocol.version=2")
	}

	// Explicitly disable credential helper, otherwise Git credentials might leak
	if DefaultFeatures().CheckVersionAtLeast("2.9") {
		globalCommandArgs = append(globalCommandArgs, "-c", "credential.helper=")
	}

	if setting.LFS.StartServer {
		if !DefaultFeatures().CheckVersionAtLeast("2.1.2") {
			return errors.New("LFS server support requires Git >= 2.1.2")
		}
		globalCommandArgs = append(globalCommandArgs, "-c", "filter.lfs.required=", "-c", "filter.lfs.smudge=", "-c", "filter.lfs.clean=")
	}

	return syncGitConfig()
}

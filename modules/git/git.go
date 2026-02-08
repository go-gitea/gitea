// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/tempdir"

	"github.com/hashicorp/go-version"
)

const RequiredVersion = "2.6.0" // the minimum Git version required

type Features struct {
	gitVersion *version.Version

	UsingGogit                 bool
	SupportProcReceive         bool           // >= 2.29
	SupportHashSha256          bool           // >= 2.42, SHA-256 repositories no longer an ‘experimental curiosity’
	SupportedObjectFormats     []ObjectFormat // sha1, sha256
	SupportCheckAttrOnBare     bool           // >= 2.40
	SupportCatFileBatchCommand bool           // >= 2.36, support `git cat-file --batch-command`
	SupportGitMergeTree        bool           // >= 2.40 // we also need "--merge-base"
}

var defaultFeatures *Features

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
		if err := InitSimple(); err != nil {
			log.Fatal("git.InitSimple failed: %v", err)
		}
	}
	return defaultFeatures
}

func loadGitVersionFeatures() (*Features, error) {
	stdout, _, runErr := gitcmd.NewCommand("version").RunStdString(context.Background())
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
	features.SupportCheckAttrOnBare = features.CheckVersionAtLeast("2.40")
	features.SupportCatFileBatchCommand = features.CheckVersionAtLeast("2.36")
	features.SupportGitMergeTree = features.CheckVersionAtLeast("2.40") // we also need "--merge-base"
	return features, nil
}

func parseGitVersionLine(s string) (*version.Version, error) {
	fields := strings.Fields(s)
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid git version: %q", s)
	}

	// version output is like: "git version {versionString}"
	// versionString can be:
	// * "2.5.3"
	// * "2.29.3.windows.1"
	// * "2.28.0.618.gf4bc123cb7": https://github.com/go-gitea/gitea/issues/12731
	versionString := fields[2]
	versionFields := strings.Split(versionString, ".")
	if len(versionFields) > 3 {
		versionFields = versionFields[:3]
	}
	return version.NewVersion(strings.Join(versionFields, "."))
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

// InitSimple initializes git module with a very simple step, no config changes, no global command arguments.
// This method doesn't change anything to filesystem. At the moment, it is only used by some Gitea sub-commands.
func InitSimple() error {
	if setting.Git.HomePath == "" {
		return errors.New("unable to init Git's HomeDir, incorrect initialization of the setting and git modules")
	}

	if defaultFeatures != nil && (!setting.IsProd || setting.IsInTesting) {
		log.Warn("git module has been initialized already, duplicate init may work but it's better to fix it")
	}

	if err := gitcmd.SetExecutablePath(setting.Git.Path); err != nil {
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
		_ = os.Setenv("GNUPGHOME", filepath.Join(gitcmd.HomeDir(), ".gnupg"))
	}
	return nil
}

// InitFull initializes git module with version check and change global variables, sync gitconfig.
// It should only be called once at the beginning of the program initialization (TestMain/GlobalInitInstalled) as this code makes unsynchronized changes to variables.
func InitFull() (err error) {
	if err = InitSimple(); err != nil {
		return err
	}

	if setting.LFS.StartServer {
		if !DefaultFeatures().CheckVersionAtLeast("2.1.2") {
			return errors.New("LFS server support requires Git >= 2.1.2")
		}
	}

	return syncGitConfig(context.Background())
}

// RunGitTests helps to init the git module and run tests.
// FIXME: GIT-PACKAGE-DEPENDENCY: the dependency is not right, setting.Git.HomePath is initialized in this package but used in gitcmd package
func RunGitTests(m interface{ Run() int }) {
	fatalf := func(exitCode int, format string, args ...any) {
		_, _ = fmt.Fprintf(os.Stderr, format, args...)
		os.Exit(exitCode)
	}
	gitHomePath, cleanup, err := tempdir.OsTempDir("gitea-test").MkdirTempRandom("git-home")
	if err != nil {
		fatalf(1, "unable to create temp dir: %s", err.Error())
	}
	defer cleanup()

	setting.Git.HomePath = gitHomePath
	if err = InitFull(); err != nil {
		fatalf(1, "failed to call Init: %s", err.Error())
	}
	if exitCode := m.Run(); exitCode != 0 {
		fatalf(exitCode, "run test failed, ExitCode=%d", exitCode)
	}
}

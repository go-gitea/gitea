// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/hashicorp/go-version"
)

var (
	// GitVersionRequired is the minimum Git version required
	// At the moment, all code for git 1.x are not changed, if some users want to test with old git client
	// or bypass the check, they still have a chance to edit this variable manually.
	// If everything works fine, the code for git 1.x could be removed in a separate PR before 1.17 frozen.
	GitVersionRequired = "2.0.0"

	// GitExecutable is the command name of git
	// Could be updated to an absolute path while initialization
	GitExecutable = "git"

	// GlobalConfigFile is the global config file used by Gitea internally
	GlobalConfigFile string

	// DefaultContext is the default context to run git commands in
	// will be overwritten by InitWithConfigSync with HammerContext
	DefaultContext = context.Background()

	// SupportProcReceive version >= 2.29.0
	SupportProcReceive bool

	gitVersion *version.Version
)

// loadGitVersion returns current Git version from shell. Internal usage only.
func loadGitVersion() (*version.Version, error) {
	// doesn't need RWMutex because its exec by Init()
	if gitVersion != nil {
		return gitVersion, nil
	}

	stdout, _, runErr := NewCommand(DefaultContext, "version").RunStdString(nil)
	if runErr != nil {
		return nil, runErr
	}

	fields := strings.Fields(stdout)
	if len(fields) < 3 {
		return nil, fmt.Errorf("invalid git version output: %s", stdout)
	}

	var versionString string

	// Handle special case on Windows.
	i := strings.Index(fields[2], "windows")
	if i >= 1 {
		versionString = fields[2][:i-1]
	} else {
		versionString = fields[2]
	}

	var err error
	gitVersion, err = version.NewVersion(versionString)
	return gitVersion, err
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

	_, err = loadGitVersion()
	if err != nil {
		return fmt.Errorf("unable to load git version: %w", err)
	}

	versionRequired, err := version.NewVersion(GitVersionRequired)
	if err != nil {
		return err
	}

	if gitVersion.LessThan(versionRequired) {
		moreHint := "get git: https://git-scm.com/download/"
		if runtime.GOOS == "linux" {
			// there are a lot of CentOS/RHEL users using old git, so we add a special hint for them
			if _, err = os.Stat("/etc/redhat-release"); err == nil {
				// ius.io is the recommended official(git-scm.com) method to install git
				moreHint = "get git: https://git-scm.com/download/linux and https://ius.io"
			}
		}
		return fmt.Errorf("installed git version %q is not supported, Gitea requires git version >= %q, %s", gitVersion.Original(), GitVersionRequired, moreHint)
	}

	return nil
}

// VersionInfo returns git version information
func VersionInfo() string {
	if gitVersion == nil {
		return "(git not found)"
	}
	format := "%s"
	args := []interface{}{gitVersion.Original()}
	// Since git wire protocol has been released from git v2.18
	if setting.Git.EnableAutoGitWireProtocol && CheckGitVersionAtLeast("2.18") == nil {
		format += ", Wire Protocol %s Enabled"
		args = append(args, "Version 2") // for focus color
	}

	return fmt.Sprintf(format, args...)
}

// InitSimple initializes git module with a very simple step, no config changes, no global command arguments.
// This method doesn't change anything to filesystem
func InitSimple(ctx context.Context) error {
	DefaultContext = ctx

	if setting.Git.Timeout.Default > 0 {
		defaultCommandExecutionTimeout = time.Duration(setting.Git.Timeout.Default) * time.Second
	}

	if setting.RepoRootPath == "" {
		return errors.New("RepoRootPath is empty, git module needs that setting before initialization")
	}

	GlobalConfigFile = setting.RepoRootPath + "/gitconfig"

	if err := SetExecutablePath(setting.Git.Path); err != nil {
		return err
	}

	// force cleanup args
	globalCommandArgs = []string{}

	return nil
}

// InitWithConfigSync initializes git module. This method may create directories or write files into filesystem
func InitWithConfigSync(ctx context.Context) error {
	err := InitSimple(ctx)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(setting.RepoRootPath, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create directory %s, err:%w", setting.RepoRootPath, err)
	}

	if CheckGitVersionAtLeast("2.9") == nil {
		// Explicitly disable credential helper, otherwise Git credentials might leak
		globalCommandArgs = append(globalCommandArgs, "-c", "credential.helper=")
	}

	// Since git wire protocol has been released from git v2.18
	if setting.Git.EnableAutoGitWireProtocol && CheckGitVersionAtLeast("2.18") == nil {
		globalCommandArgs = append(globalCommandArgs, "-c", "protocol.version=2")
	}

	// By default partial clones are disabled, enable them from git v2.22
	if !setting.Git.DisablePartialClone && CheckGitVersionAtLeast("2.22") == nil {
		globalCommandArgs = append(globalCommandArgs, "-c", "uploadpack.allowfilter=true", "-c", "uploadpack.allowAnySHA1InWant=true")
	}

	// Git requires setting user.name and user.email in order to commit changes - if they're not set just add some defaults
	for configKey, defaultValue := range map[string]string{"user.name": "Gitea", "user.email": "gitea@fake.local"} {
		if err := configSet(configKey, defaultValue); err != nil {
			return err
		}
	}

	// Set git some configurations - these must be set to these values for gitea to work correctly
	if err := configSet("core.quotePath", "false"); err != nil {
		return err
	}

	if CheckGitVersionAtLeast("2.10") == nil {
		if err := configSet("receive.advertisePushOptions", "true"); err != nil {
			return err
		}
	}

	if CheckGitVersionAtLeast("2.18") == nil {
		if err := configSet("core.commitGraph", "true"); err != nil {
			return err
		}
		if err := configSet("gc.writeCommitGraph", "true"); err != nil {
			return err
		}
	}

	if CheckGitVersionAtLeast("2.29") == nil {
		// set support for AGit flow
		if err := configAddNonExist("receive.procReceiveRefs", "refs/for"); err != nil {
			return err
		}
		SupportProcReceive = true
	} else {
		if err := configUnsetAll("receive.procReceiveRefs", "refs/for"); err != nil {
			return err
		}
		SupportProcReceive = false
	}

	if CheckGitVersionAtLeast("2.35.2") == nil {
		// since Git 2.35.2, git adds a protection for CVE-2022-24765, the protection denies the git directories which are not owned by current user
		// however, some docker users and samba users (maybe more, issue #19455) have difficulty to set their Gitea git repositories to the correct owner.
		// the reason behind the problem is: docker/samba uses some uid-mapping mechanism, which are unstable/unfixable in some cases.
		// now Gitea always use its customized git config file, and all the accesses to the git repositories can be managed,
		// so it's safe to set "safe.directory=*" for internal usage only.
		if err := configSet("safe.directory", "*"); err != nil {
			return err
		}
	}

	if runtime.GOOS == "windows" {
		if err := configSet("core.longpaths", "true"); err != nil {
			return err
		}
		if setting.Git.DisableCoreProtectNTFS {
			globalCommandArgs = append(globalCommandArgs, "-c", "core.protectntfs=false")
		}
	}

	return nil
}

// CheckGitVersionAtLeast check git version is at least the constraint version
func CheckGitVersionAtLeast(atLeast string) error {
	if _, err := loadGitVersion(); err != nil {
		return err
	}
	atLeastVersion, err := version.NewVersion(atLeast)
	if err != nil {
		return err
	}
	if gitVersion.Compare(atLeastVersion) < 0 {
		return fmt.Errorf("installed git binary version %s is not at least %s", gitVersion.Original(), atLeast)
	}
	return nil
}

func configSet(key, value string) error {
	stdout, _, err := NewCommand(DefaultContext, "config", "--get", key).RunStdString(nil)
	if err != nil && !err.IsExitCode(1) {
		return fmt.Errorf("failed to get git config %s, err:%w", key, err)
	}

	currValue := strings.TrimSpace(stdout)
	if currValue == value {
		return nil
	}

	_, _, err = NewCommand(DefaultContext, "config", "--global", key, value).RunStdString(nil)
	if err != nil {
		return fmt.Errorf("failed to set git global config %s, err:%w", key, err)
	}

	return nil
}

func configAddNonExist(key, value string) error {
	_, _, err := NewCommand(DefaultContext, "config", "--get", key).RunStdString(nil)
	if err == nil {
		// already exist
		return nil
	}
	if err.IsExitCode(1) {
		// not exist, add new config
		_, _, err = NewCommand(DefaultContext, "config", "--global", key, value).RunStdString(nil)
		if err != nil {
			return fmt.Errorf("failed to set git global config %s, err:%w", key, err)
		}
		return nil
	}
	return fmt.Errorf("failed to get git config %s, err:%w", key, err)
}

func configUnsetAll(key, valueRegex string) error {
	_, _, err := NewCommand(DefaultContext, "config", "--get", key).RunStdString(nil)
	if err == nil {
		// exist, need to remove
		_, _, err = NewCommand(DefaultContext, "config", "--global", "--unset-all", key, valueRegex).RunStdString(nil)
		if err != nil {
			return fmt.Errorf("failed to unset git global config %s, err:%w", key, err)
		}
		return nil
	}
	if err.IsExitCode(1) {
		// not exist
		return nil
	}
	return fmt.Errorf("failed to get git config %s, err:%w", key, err)
}

// Fsck verifies the connectivity and validity of the objects in the database
func Fsck(ctx context.Context, repoPath string, timeout time.Duration, args ...string) error {
	return NewCommand(ctx, "fsck").AddArguments(args...).Run(&RunOpts{Timeout: timeout, Dir: repoPath})
}

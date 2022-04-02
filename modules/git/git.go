// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	git_cmd "code.gitea.io/gitea/modules/git/cmd"
	"code.gitea.io/gitea/modules/setting"

	"github.com/hashicorp/go-version"
)

var (
	// GitVersionRequired is the minimum Git version required
	// At the moment, all code for git 1.x are not changed, if some users want to test with old git client
	// or bypass the check, they still have a chance to edit this variable manually.
	// If everything works fine, the code for git 1.x could be removed in a separate PR before 1.17 frozen.
	GitVersionRequired = "2.0.0"

	// DefaultContext is the default context to run git commands in
	// will be overwritten by Init with HammerContext
	DefaultContext = context.Background()

	gitVersion *version.Version

	// SupportProcReceive version >= 2.29.0
	SupportProcReceive bool
)

// LocalVersion returns current Git version from shell.
func LocalVersion() (*version.Version, error) {
	if err := LoadGitVersion(); err != nil {
		return nil, err
	}
	return gitVersion, nil
}

// LoadGitVersion returns current Git version from shell.
func LoadGitVersion() error {
	// doesn't need RWMutex because its exec by Init()
	if gitVersion != nil {
		return nil
	}

	stdout, _, runErr := NewCommand(context.Background(), "version").RunStdString(nil)
	if runErr != nil {
		return runErr
	}

	fields := strings.Fields(stdout)
	if len(fields) < 3 {
		return fmt.Errorf("not enough output: %s", stdout)
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
	return err
}

// checkGitVersion checks the file permission and version.
func checkGitVersion() error {
	err := LoadGitVersion()
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
	format := "Git Version: %s"
	args := []interface{}{gitVersion.Original()}
	// Since git wire protocol has been released from git v2.18
	if setting.Git.EnableAutoGitWireProtocol && CheckGitVersionAtLeast("2.18") == nil {
		format += ", Wire Protocol %s Enabled"
		args = append(args, "Version 2") // for focus color
	}

	return fmt.Sprintf(format, args...)
}

// Init initializes git module
func Init(ctx context.Context) error {
	DefaultContext = ctx

	if err := checkGitVersion(); err != nil {
		return err
	}

	// force cleanup args
	globalCommandArgs = []string{}

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
		if err := checkAndSetConfig(configKey, defaultValue, false); err != nil {
			return err
		}
	}

	// Set git some configurations - these must be set to these values for gitea to work correctly
	if err := checkAndSetConfig("core.quotePath", "false", true); err != nil {
		return err
	}

	if CheckGitVersionAtLeast("2.10") == nil {
		if err := checkAndSetConfig("receive.advertisePushOptions", "true", true); err != nil {
			return err
		}
	}

	if CheckGitVersionAtLeast("2.18") == nil {
		if err := checkAndSetConfig("core.commitGraph", "true", true); err != nil {
			return err
		}
		if err := checkAndSetConfig("gc.writeCommitGraph", "true", true); err != nil {
			return err
		}
	}

	if CheckGitVersionAtLeast("2.29") == nil {
		// set support for AGit flow
		if err := checkAndAddConfig("receive.procReceiveRefs", "refs/for"); err != nil {
			return err
		}
		SupportProcReceive = true
	} else {
		if err := checkAndRemoveConfig("receive.procReceiveRefs", "refs/for"); err != nil {
			return err
		}
		SupportProcReceive = false
	}

	if runtime.GOOS == "windows" {
		if err := checkAndSetConfig("core.longpaths", "true", true); err != nil {
			return err
		}
	}
	if setting.Git.DisableCoreProtectNTFS {
		if err := checkAndSetConfig("core.protectntfs", "false", true); err != nil {
			return err
		}
		globalCommandArgs = append(globalCommandArgs, "-c", "core.protectntfs=false")
	}
	return nil
}

// CheckGitVersionAtLeast check git version is at least the constraint version
func CheckGitVersionAtLeast(atLeast string) error {
	if err := LoadGitVersion(); err != nil {
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

var gitConfigLock sync.Mutex

// checkAndSetConfig to avoid config conflict, only allow one go routine call at the same time
func checkAndSetConfig(key, defaultValue string, forceToDefault bool) error {
	gitConfigLock.Lock()
	defer gitConfigLock.Unlock()

	stdout := strings.Builder{}
	stderr := strings.Builder{}
	if err := NewCommand(DefaultContext, "config", "--get", key).
		SetDescription("git.Init(get setting)").
		Run(&git_cmd.RunOpts{
			Stdout: &stdout,
			Stderr: &stderr,
		}); err != nil {
		eerr, ok := err.(*exec.ExitError)
		if !ok || eerr.ExitCode() != 1 {
			return fmt.Errorf("failed to get git %s(%v) errType %T: %s", key, err, err, stderr.String())
		}
	}

	currValue := strings.TrimSpace(stdout.String())
	if currValue == defaultValue || (!forceToDefault && len(currValue) > 0) {
		return nil
	}

	stderr.Reset()

	if err := NewCommand(DefaultContext, "config", "--global", key, defaultValue).
		SetDescription(fmt.Sprintf("git.Init(set %s)", key)).
		Run(&git_cmd.RunOpts{
			Stderr: &stderr,
		}); err != nil {
		return fmt.Errorf("failed to set git %s(%s): %s", key, err, stderr.String())
	}

	return nil
}

func checkAndAddConfig(key, value string) error {
	gitConfigLock.Lock()
	defer gitConfigLock.Unlock()

	stdout := strings.Builder{}
	stderr := strings.Builder{}
	if err := NewCommand(DefaultContext, "config", "--get", key).
		SetDescription("git.Init(get setting)").
		Run(&git_cmd.RunOpts{
			Stdout: &stdout,
			Stderr: &stderr,
		}); err != nil {
		eerr, ok := err.(*exec.ExitError)
		if !ok || eerr.ExitCode() != 1 {
			return fmt.Errorf("failed to get git %s(%v) errType %T: %s", key, err, err, stderr.String())
		}
		if eerr.ExitCode() == 1 {
			stderr.Reset()
			if err := NewCommand(DefaultContext, "config", "--global", "--add", key, value).
				SetDescription(fmt.Sprintf("git.Init(set %s)", key)).
				Run(&git_cmd.RunOpts{
					Stderr: &stderr,
				}); err != nil {
				return fmt.Errorf("failed to set git %s(%s): %s", key, err, stderr.String())
			}
		}
	}

	return nil
}

func checkAndRemoveConfig(key, value string) error {
	gitConfigLock.Lock()
	defer gitConfigLock.Unlock()

	stderr := strings.Builder{}
	if err := NewCommand(DefaultContext, "config", "--get", key, value).
		SetDescription("git.Init(get setting)").
		Run(&git_cmd.RunOpts{
			Stderr: &stderr,
		}); err != nil {
		eerr, ok := err.(*exec.ExitError)
		if !ok || eerr.ExitCode() != 1 {
			return fmt.Errorf("failed to get git %s(%v) errType %T: %s", key, err, err, stderr.String())
		}
		if eerr.ExitCode() == 1 {
			return nil
		}
	}

	stderr.Reset()
	if err := NewCommand(DefaultContext, "config", "--global", "--unset-all", key, value).
		SetDescription(fmt.Sprintf("git.Init(set %s)", key)).
		Run(&git_cmd.RunOpts{
			Stderr: &stderr,
		}); err != nil {
		return fmt.Errorf("failed to set git %s(%s): %s", key, err, stderr.String())
	}

	return nil
}

// Fsck verifies the connectivity and validity of the objects in the database
func Fsck(ctx context.Context, repoPath string, timeout time.Duration, args ...string) error {
	return NewCommand(ctx, "fsck").AddArguments(args...).Run(&RunOpts{Timeout: timeout, Dir: repoPath})
}

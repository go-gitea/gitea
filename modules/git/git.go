// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/process"

	"github.com/mcuadros/go-version"
)

// Version return this package's current version
func Version() string {
	return "0.4.2"
}

var (
	// Debug enables verbose logging on everything.
	// This should be false in case Gogs starts in SSH mode.
	Debug = false
	// Prefix the log prefix
	Prefix = "[git-module] "
	// GitVersionRequired is the minimum Git version required
	GitVersionRequired = "1.7.2"

	// GitExecutable is the command name of git
	// Could be updated to an absolute path while initialization
	GitExecutable = "git"

	// DefaultContext is the default context to run git commands in
	DefaultContext = context.Background()

	gitVersion string
)

func log(format string, args ...interface{}) {
	if !Debug {
		return
	}

	fmt.Print(Prefix)
	if len(args) == 0 {
		fmt.Println(format)
	} else {
		fmt.Printf(format+"\n", args...)
	}
}

// BinVersion returns current Git version from shell.
func BinVersion() (string, error) {
	if len(gitVersion) > 0 {
		return gitVersion, nil
	}

	stdout, err := NewCommand("version").Run()
	if err != nil {
		return "", err
	}

	fields := strings.Fields(stdout)
	if len(fields) < 3 {
		return "", fmt.Errorf("not enough output: %s", stdout)
	}

	// Handle special case on Windows.
	i := strings.Index(fields[2], "windows")
	if i >= 1 {
		gitVersion = fields[2][:i-1]
		return gitVersion, nil
	}

	gitVersion = fields[2]
	return gitVersion, nil
}

// SetExecutablePath changes the path of git executable and checks the file permission and version.
func SetExecutablePath(path string) error {
	// If path is empty, we use the default value of GitExecutable "git" to search for the location of git.
	if path != "" {
		GitExecutable = path
	}
	absPath, err := exec.LookPath(GitExecutable)
	if err != nil {
		return fmt.Errorf("Git not found: %v", err)
	}
	GitExecutable = absPath

	gitVersion, err := BinVersion()
	if err != nil {
		return fmt.Errorf("Git version missing: %v", err)
	}
	if version.Compare(gitVersion, GitVersionRequired, "<") {
		return fmt.Errorf("Git version not supported. Requires version > %v", GitVersionRequired)
	}

	return nil
}

// Init initializes git module
func Init(ctx context.Context) error {
	DefaultContext = ctx
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

	if version.Compare(gitVersion, "2.18", ">=") {
		if err := checkAndSetConfig("core.commitGraph", "true", true); err != nil {
			return err
		}
		if err := checkAndSetConfig("gc.writeCommitGraph", "true", true); err != nil {
			return err
		}
	}

	if runtime.GOOS == "windows" {
		if err := checkAndSetConfig("core.longpaths", "true", true); err != nil {
			return err
		}
	}
	return nil
}

func checkAndSetConfig(key, defaultValue string, forceToDefault bool) error {
	stdout, stderr, err := process.GetManager().Exec("git.Init(get setting)", GitExecutable, "config", "--get", key)
	if err != nil {
		perr, ok := err.(*process.Error)
		if !ok {
			return fmt.Errorf("Failed to get git %s(%v) errType %T: %s", key, err, err, stderr)
		}
		eerr, ok := perr.Err.(*exec.ExitError)
		if !ok || eerr.ExitCode() != 1 {
			return fmt.Errorf("Failed to get git %s(%v) errType %T: %s", key, err, err, stderr)
		}
	}

	currValue := strings.TrimSpace(stdout)

	if currValue == defaultValue || (!forceToDefault && len(currValue) > 0) {
		return nil
	}

	if _, stderr, err = process.GetManager().Exec(fmt.Sprintf("git.Init(set %s)", key), "git", "config", "--global", key, defaultValue); err != nil {
		return fmt.Errorf("Failed to set git %s(%s): %s", key, err, stderr)
	}

	return nil
}

// Fsck verifies the connectivity and validity of the objects in the database
func Fsck(ctx context.Context, repoPath string, timeout time.Duration, args ...string) error {
	// Make sure timeout makes sense.
	if timeout <= 0 {
		timeout = -1
	}
	_, err := NewCommandContext(ctx, "fsck").AddArguments(args...).RunInDirTimeout(timeout, repoPath)
	return err
}

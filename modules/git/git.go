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
	// Git requires setting user.name and user.email in order to commit changes.
	for configKey, defaultValue := range map[string]string{"user.name": "Gitea", "user.email": "gitea@fake.local"} {
		if stdout, stderr, err := process.GetManager().Exec("git.Init(get setting)", GitExecutable, "config", "--get", configKey); err != nil || strings.TrimSpace(stdout) == "" {
			// ExitError indicates this config is not set
			if _, ok := err.(*exec.ExitError); ok || strings.TrimSpace(stdout) == "" {
				if _, stderr, gerr := process.GetManager().Exec("git.Init(set "+configKey+")", "git", "config", "--global", configKey, defaultValue); gerr != nil {
					return fmt.Errorf("Failed to set git %s(%s): %s", configKey, gerr, stderr)
				}
			} else {
				return fmt.Errorf("Failed to get git %s(%s): %s", configKey, err, stderr)
			}
		}
	}

	// Set git some configurations.
	if _, stderr, err := process.GetManager().Exec("git.Init(git config --global core.quotepath false)",
		GitExecutable, "config", "--global", "core.quotepath", "false"); err != nil {
		return fmt.Errorf("Failed to execute 'git config --global core.quotepath false': %s", stderr)
	}

	if version.Compare(gitVersion, "2.18", ">=") {
		if _, stderr, err := process.GetManager().Exec("git.Init(git config --global core.commitGraph true)",
			GitExecutable, "config", "--global", "core.commitGraph", "true"); err != nil {
			return fmt.Errorf("Failed to execute 'git config --global core.commitGraph true': %s", stderr)
		}

		if _, stderr, err := process.GetManager().Exec("git.Init(git config --global gc.writeCommitGraph true)",
			GitExecutable, "config", "--global", "gc.writeCommitGraph", "true"); err != nil {
			return fmt.Errorf("Failed to execute 'git config --global gc.writeCommitGraph true': %s", stderr)
		}
	}

	if runtime.GOOS == "windows" {
		if _, stderr, err := process.GetManager().Exec("git.Init(git config --global core.longpaths true)",
			GitExecutable, "config", "--global", "core.longpaths", "true"); err != nil {
			return fmt.Errorf("Failed to execute 'git config --global core.longpaths true': %s", stderr)
		}
	}
	return nil
}

// Fsck verifies the connectivity and validity of the objects in the database
func Fsck(repoPath string, timeout time.Duration, args ...string) error {
	// Make sure timeout makes sense.
	if timeout <= 0 {
		timeout = -1
	}
	_, err := NewCommand("fsck").AddArguments(args...).RunInDirTimeout(timeout, repoPath)
	return err
}

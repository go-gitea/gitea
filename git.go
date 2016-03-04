// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"strings"
	"time"
)

const _VERSION = "0.2.9"

func Version() string {
	return _VERSION
}

var (
	// Debug enables verbose logging on everything.
	// This should be false in case Gogs starts in SSH mode.
	Debug  = false
	Prefix = "[git-module] "
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

var gitVersion string

// Version returns current Git version from shell.
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

func init() {
	BinVersion()
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

// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"code.gitea.io/gitea/modules/process"
)

// GetInterdiff get the difference of two diffs.
func GetInterdiff(diff1, diff2 string) (string, error) {
	f1 := "/dev/null"
	f2 := "/dev/null"

	if diff1 != "" {
		f, err := ioutil.TempFile("", "d1")
		if err != nil {
			return "", err
		}
		f1 = f.Name()
		defer os.Remove(f1)

		_, err = f.WriteString(diff1)

		if err != nil {
			return "", err
		}
	}

	if diff2 != "" {
		f, err := ioutil.TempFile("", "d2")
		if err != nil {
			return "", err
		}

		_, err = f.WriteString(diff2)

		if err != nil {
			return "", err
		}

		f2 = f.Name()
		defer os.Remove(f2)
	}

	ctx, cancel := context.WithCancel(DefaultContext)
	defer cancel()
	var cmd = exec.CommandContext(ctx, Interdiff, "-q", f1, f2)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("Start: %v", err)
	}

	pid := process.GetManager().Add("GetInterdiff ", cancel)
	defer process.GetManager().Remove(pid)

	buf := new(strings.Builder)
	_, err = io.Copy(buf, stdout)
	if err != nil {
		return "", err
	}

	if err = cmd.Wait(); err != nil {
		return "", fmt.Errorf("Wait: %v", err)
	}

	return buf.String(), nil
}

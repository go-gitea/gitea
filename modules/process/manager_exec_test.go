// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func retry(t *testing.T, fun func() error, tries int) {
	var err interface{}
	for i := 0; i < tries; i++ {
		err = fun()
		if err == nil {
			return
		}
		<-time.After(1 * time.Second)
	}
	assert.Fail(t, fmt.Sprintf("Retry: failed \n%v", err))
}

func TestManagerKillGrandChildren(t *testing.T) {
	tmp := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	pm := &Manager{
		processMap: make(map[IDType]*process),
		next:       1,
	}

	go func() {
		// blocks forever because of the firewall at 4.4.4.4
		_, _, _ = pm.ExecDir(ctx, -1, tmp, "GIT description", "git", "clone", "https://4.4.4.4", "something")
	}()

	// the git clone process forks a grand child git-remote-https, wait for it
	pattern := "git-remote-https origin https://4.4.4.4"
	ps := func() string {
		cmd := exec.Command("ps", "-x", "-o", "pid,ppid,pgid,args")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err)
		return string(output)
	}

	retry(t, func() error {
		out := ps()
		if !strings.Contains(out, pattern) {
			return fmt.Errorf(out + "Does not contain " + pattern)
		}
		return nil
	}, 5)

	// canceling the parent context will cause the child process to be killed
	cancel()
	<-ctx.Done()

	// wait for the git-remote-https grand child process to terminate
	retry(t, func() error {
		out := ps()
		if strings.Contains(out, pattern) {
			return fmt.Errorf(out + "Contains " + pattern)
		}
		return nil
	}, 5)
}

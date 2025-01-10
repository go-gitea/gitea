// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getWhoamiOutput() (string, error) {
	output, err := exec.Command("whoami").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func TestCurrentUsername(t *testing.T) {
	user := CurrentUsername()
	require.NotEmpty(t, user)

	// Windows whoami is weird, so just skip remaining tests
	if runtime.GOOS == "windows" {
		t.Skip("skipped test because of weird whoami on Windows")
	}
	whoami, err := getWhoamiOutput()
	require.NoError(t, err)

	user = CurrentUsername()
	assert.Equal(t, whoami, user)

	t.Setenv("USER", "spoofed")
	user = CurrentUsername()
	assert.Equal(t, whoami, user)
}

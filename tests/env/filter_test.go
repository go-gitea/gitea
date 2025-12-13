// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package env

import (
	"os"
	"testing"
)

func TestFilter(t *testing.T) {
	t.Setenv("GITEA_FOO", "bar")
	t.Setenv("FOO", "bar")
	Filter([]string{}, []string{"GITEA_"})
	if os.Getenv("GITEA_FOO") != "" {
		t.FailNow()
	}
	if os.Getenv("FOO") != "bar" {
		t.FailNow()
	}

	t.Setenv("GITEA_TEST_FOO", "bar")
	t.Setenv("GITEA_BAR", "foo")
	t.Setenv("GITEA_BAR_BAZ", "foo")
	t.Setenv("GITEA_BAZ", "huz")
	Filter([]string{"GITEA_TEST_", "GITEA_BAR="}, []string{"GITEA_"})
	if os.Getenv("GITEA_BAR") != "foo" {
		t.Fail()
	}
	if os.Getenv("GITEA_TEST_FOO") != "bar" {
		t.Fail()
	}
	if os.Getenv("GITEA_BAZ") != "" {
		t.Fail()
	}
	if os.Getenv("GITEA_BAR_BAZ") != "" {
		t.Fail()
	}
}

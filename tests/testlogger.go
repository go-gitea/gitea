// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tests

import (
	"testing"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/testlogger"
)

func PrintCurrentTest(t testing.TB, skip ...int) func() {
	if len(skip) == 1 {
		skip = []int{skip[0] + 1}
	}
	return testlogger.PrintCurrentTest(t, skip...)
}

// Printf takes a format and args and prints the string to os.Stdout
func Printf(format string, args ...interface{}) {
	testlogger.Printf(format, args...)
}

func init() {
	log.Register("test", testlogger.NewTestLogger)
}

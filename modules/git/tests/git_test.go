// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tests

import (
	"context"
	"fmt"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
)

const testReposDir = "repos/"
const benchmarkReposDir = "../benchmark/repos/"

func fatalTestError(fmtStr string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, fmtStr, args...)
	os.Exit(1)
}

func TestMain(m *testing.M) {
	log.NewLogger(0, "console", "console", fmt.Sprintf(`{"level": "info", "colorize": %t, "stacktraceLevel": "none"}`, false))
	if err := git.Init(context.Background()); err != nil {
		fatalTestError("Init failed: %v", err)
	}

	exitStatus := m.Run()
	os.Exit(exitStatus)
}

func RunTestPerProvider(t *testing.T, fn func(s service.GitService, t *testing.T)) {
	providers := git.GetServiceProviders()
	for _, providerName := range providers {
		provider := git.GetServiceProvider(providerName)
		t.Run(fmt.Sprintf("%s_%s", t.Name(), providerName), func(t *testing.T) {
			fn(provider, t)
		})
	}
}

func RunBenchmarkPerProvider(b *testing.B, fn func(s service.GitService, b *testing.B)) {
	providers := git.GetServiceProviders()
	for _, providerName := range providers {
		provider := git.GetServiceProvider(providerName)
		b.Run(fmt.Sprintf("%s_%s", b.Name(), providerName), func(b *testing.B) {
			fn(provider, b)
		})
	}
}

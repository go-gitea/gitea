// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package customlint

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("customlint", func(any) (register.LinterPlugin, error) {
		return &plugin{}, nil
	})
}

type plugin struct{}

func (*plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{
		goheaderAnalyzer,
	}, nil
}

func (*plugin) GetLoadMode() string {
	return register.LoadModeSyntax
}

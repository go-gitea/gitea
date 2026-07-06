// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/modules/gitrepo"
)

var linterConfigs = []string{
	".golangci.yml", ".golangci.yaml",
	".eslintrc", ".eslintrc.json", ".eslintrc.js", ".eslintrc.yaml",
	".jshintrc",
	"tsconfig.json", // has compiler options that affect linting
	"ruff.toml", ".ruff.toml", "pyproject.toml", // ruff/pylint
	".pylintrc",
}

// DetectLinterConfigs checks the repo's default branch for linter configuration files.
func DetectLinterConfigs(ctx context.Context, repo gitrepo.Repository) string {
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return ""
	}
	defer gitRepo.Close()

	defaultBranch, err := gitrepo.GetDefaultBranch(ctx, repo)
	if err != nil || defaultBranch == "" {
		return ""
	}

	commit, err := gitRepo.GetBranchCommit(defaultBranch)
	if err != nil {
		return ""
	}

	var found []string
	for _, cfg := range linterConfigs {
		content, err := commit.GetFileContent(cfg, 500) // just check first 500 bytes
		if err == nil && content != "" {
			found = append(found, cfg)
		}
	}

	if len(found) == 0 {
		return ""
	}

	return fmt.Sprintf("Linter configs detected: %s\n", strings.Join(found, ", "))
}

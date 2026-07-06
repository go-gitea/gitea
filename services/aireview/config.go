// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package aireview

import (
	"context"
	"fmt"

	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/log"

	"gopkg.in/yaml.v3"
)

// RepoConfig holds per-repository AI review configuration, loaded from .gitea/ai-review.yaml.
type RepoConfig struct {
	Enabled          *bool             `yaml:"enabled"`
	SystemPrompt     *string           `yaml:"system_prompt"`
	ExcludePaths     []string          `yaml:"exclude_paths"`
	PathInstructions []PathInstruction `yaml:"path_instructions"`
	CustomChecks     []string          `yaml:"custom_checks"`
}

// PathInstruction specifies custom review instructions for files matching a path glob.
type PathInstruction struct {
	Path         string `yaml:"path"`
	Instructions string `yaml:"instructions"`
}

const repoConfigFile = ".gitea/ai-review.yaml"

// defaultBranchForConfig is the branch from which the per-repo config is read.
const defaultBranchForConfig = ""

// LoadRepoConfig reads the per-repository AI review config from the default branch.
func LoadRepoConfig(ctx context.Context, repo gitrepo.Repository) (*RepoConfig, error) {
	gitRepo, err := gitrepo.OpenRepository(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("open git repo: %w", err)
	}
	defer gitRepo.Close()

	defaultBranch, err := gitrepo.GetDefaultBranch(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("get default branch: %w", err)
	}
	if defaultBranch == "" {
		return nil, nil
	}

	commit, err := gitRepo.GetBranchCommit(defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("get branch commit: %w", err)
	}

	content, err := commit.GetFileContent(repoConfigFile, 1<<20) // 1 MB limit
	if err != nil {
		// File doesn't exist — not an error
		return nil, nil
	}
	if content == "" {
		return nil, nil
	}

	var cfg RepoConfig
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		log.Warn("aireview: failed to parse %s in %s: %v", repoConfigFile, defaultBranch, err)
		return nil, nil
	}

	return &cfg, nil
}

// MergeRepoConfig merges per-repo config into the effective review settings.
// It returns the effective system prompt and exclude paths for this repo.
func MergeRepoConfig(globalSystemPrompt string, globalExcludePaths []string, repoCfg *RepoConfig) (systemPrompt string, excludePaths []string, pathInstructions []PathInstruction, customChecks []string) {
	systemPrompt = globalSystemPrompt
	excludePaths = globalExcludePaths

	if repoCfg == nil {
		return systemPrompt, excludePaths, pathInstructions, customChecks
	}

	if repoCfg.SystemPrompt != nil {
		systemPrompt = *repoCfg.SystemPrompt
	}
	if repoCfg.ExcludePaths != nil {
		excludePaths = repoCfg.ExcludePaths
	}
	pathInstructions = repoCfg.PathInstructions
	customChecks = repoCfg.CustomChecks
	return systemPrompt, excludePaths, pathInstructions, customChecks
}

// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
)

// PushToExternal pushes a managed repository to an external remote.
func PushToExternal(ctx context.Context, repo RepositoryFacade, opts PushOptions) error {
	return Push(ctx, gitrepo.RepoLocalPath(repo), opts)
}

type configEntry struct {
	key, value string
}

func PushToExternalWithoutTracking(ctx context.Context, repo RepositoryFacade, pushRefSpecs []string, opts PushOptions) error {
	const tempRemoteName = "gitea-push-address"
	config, _, err := gitcmd.NewCommand("config", "--null", "--get-regexp").
		AddDynamicArguments(`^remote\.` + regexp.QuoteMeta(opts.Remote) + `\.(url|pushurl)$`).
		WithRepo(repo).WithEnv(opts.Env).RunStdString(ctx)
	if err != nil {
		return err
	}
	var configs []configEntry
	for entry := range strings.SplitSeq(strings.TrimSuffix(config, "\x00"), "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		configs = append(configs, configEntry{key: "remote." + tempRemoteName + key[len("remote."+opts.Remote):], value: value})
	}
	for _, refSpec := range pushRefSpecs {
		configs = append(configs, configEntry{key: "remote." + tempRemoteName + ".push", value: refSpec})
	}
	opts.Remote = tempRemoteName
	opts.Env = appendConfigEnv(opts.Env, configs...) // Keep URL credentials out of process arguments.
	return PushToExternal(ctx, repo, opts)
}

func appendConfigEnv(env []string, configs ...configEntry) []string {
	if env == nil {
		env = os.Environ()
	}
	count := 0
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "GIT_CONFIG_COUNT="); ok {
			count, _ = strconv.Atoi(value)
		}
	}
	for _, config := range configs {
		index := strconv.Itoa(count)
		env = append(env, "GIT_CONFIG_KEY_"+index+"="+config.key, "GIT_CONFIG_VALUE_"+index+"="+config.value)
		count++
	}
	return append(env, "GIT_CONFIG_COUNT="+strconv.Itoa(count))
}

// PushManaged pushes from one managed repository to another managed repository.
func PushManaged(ctx context.Context, fromRepo, toRepo RepositoryFacade, opts PushOptions) error {
	opts.Remote = gitrepo.RepoLocalPath(toRepo)
	return Push(ctx, gitrepo.RepoLocalPath(fromRepo), opts)
}

// PushFromLocal pushes from a local path to a managed repository.
func PushFromLocal(ctx context.Context, fromLocalPath string, toRepo RepositoryFacade, opts PushOptions) error {
	opts.Remote = gitrepo.RepoLocalPath(toRepo)
	return Push(ctx, fromLocalPath, opts)
}

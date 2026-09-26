// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"os"
	"strconv"
	"strings"

	"gitea.dev/modules/git/gitrepo"
)

// PushToExternal pushes a managed repository to an external remote.
func PushToExternal(ctx context.Context, repo RepositoryFacade, opts PushOptions) error {
	return Push(ctx, gitrepo.RepoLocalPath(repo), opts)
}

// tempRemoteName is only ever defined for a single command, so it never has a fetch refspec
const tempRemoteName = "gitea-push-address"

// PushToExternalAddress pushes a managed repository to an address instead of a configured remote name,
// so git never writes the pushed values back into local refs through the remote's fetch refspec.
func PushToExternalAddress(ctx context.Context, repo RepositoryFacade, addr string, pushRefSpecs []string, opts PushOptions) error {
	configs := []ConfigEntry{{Key: "remote." + tempRemoteName + ".url", Value: addr}}
	for _, refSpec := range pushRefSpecs {
		configs = append(configs, ConfigEntry{Key: "remote." + tempRemoteName + ".push", Value: refSpec})
	}
	opts.Remote = tempRemoteName
	if DefaultFeatures().SupportConfigEnv {
		opts.Env = appendConfigEnv(opts.Env, configs...) // keep credentials in the address out of the process arguments
	} else {
		opts.Configs = configs
	}
	return PushToExternal(ctx, repo, opts)
}

// appendConfigEnv adds git config entries via GIT_CONFIG_COUNT/KEY/VALUE, keeping any existing entries.
func appendConfigEnv(env []string, configs ...ConfigEntry) []string {
	if env == nil {
		env = os.Environ()
	}
	count := 0
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GIT_CONFIG_COUNT="); ok {
			count, _ = strconv.Atoi(v)
		}
	}
	for _, c := range configs {
		n := strconv.Itoa(count)
		env = append(env, "GIT_CONFIG_KEY_"+n+"="+c.Key, "GIT_CONFIG_VALUE_"+n+"="+c.Value)
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

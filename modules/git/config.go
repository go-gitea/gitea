// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

// syncGitConfig only modifies gitconfig, won't change global variables (otherwise there will be data-race problem)
func syncGitConfig() (err error) {
	if err = os.MkdirAll(HomeDir(), os.ModePerm); err != nil {
		return fmt.Errorf("unable to prepare git home directory %s, err: %w", HomeDir(), err)
	}

	// first, write user's git config options to git config file
	// user config options could be overwritten by builtin values later, because if a value is builtin, it must have some special purposes
	for k, v := range setting.GitConfig.Options {
		if err = configSet(strings.ToLower(k), v); err != nil {
			return err
		}
	}

	// Git requires setting user.name and user.email in order to commit changes - old comment: "if they're not set just add some defaults"
	// TODO: need to confirm whether users really need to change these values manually. It seems that these values are dummy only and not really used.
	// If these values are not really used, then they can be set (overwritten) directly without considering about existence.
	for configKey, defaultValue := range map[string]string{
		"user.name":  "Gitea",
		"user.email": "gitea@fake.local",
	} {
		if err := configSetNonExist(configKey, defaultValue); err != nil {
			return err
		}
	}

	// Set git some configurations - these must be set to these values for gitea to work correctly
	if err := configSet("core.quotePath", "false"); err != nil {
		return err
	}

	if DefaultFeatures().CheckVersionAtLeast("2.10") {
		if err := configSet("receive.advertisePushOptions", "true"); err != nil {
			return err
		}
	}

	if DefaultFeatures().CheckVersionAtLeast("2.18") {
		if err := configSet("core.commitGraph", "true"); err != nil {
			return err
		}
		if err := configSet("gc.writeCommitGraph", "true"); err != nil {
			return err
		}
		if err := configSet("fetch.writeCommitGraph", "true"); err != nil {
			return err
		}
	}

	if DefaultFeatures().SupportProcReceive {
		// set support for AGit flow
		if err := configAddNonExist("receive.procReceiveRefs", "refs/for"); err != nil {
			return err
		}
	} else {
		if err := configUnsetAll("receive.procReceiveRefs", "refs/for"); err != nil {
			return err
		}
	}

	// Due to CVE-2022-24765, git now denies access to git directories which are not owned by current user.
	// However, some docker users and samba users find it difficult to configure their systems correctly,
	// so that Gitea's git repositories are owned by the Gitea user.
	// (Possibly Windows Service users - but ownership in this case should really be set correctly on the filesystem.)
	// See issue: https://github.com/go-gitea/gitea/issues/19455
	// As Gitea now always use its internal git config file, and access to the git repositories is managed through Gitea,
	// it is now safe to set "safe.directory=*" for internal usage only.
	// Although this setting is only supported by some new git versions, it is also tolerated by earlier versions
	if err := configAddNonExist("safe.directory", "*"); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if err := configSet("core.longpaths", "true"); err != nil {
			return err
		}
		if setting.Git.DisableCoreProtectNTFS {
			err = configSet("core.protectNTFS", "false")
		} else {
			err = configUnsetAll("core.protectNTFS", "false")
		}
		if err != nil {
			return err
		}
	}

	// By default partial clones are disabled, enable them from git v2.22
	if !setting.Git.DisablePartialClone && DefaultFeatures().CheckVersionAtLeast("2.22") {
		if err = configSet("uploadpack.allowfilter", "true"); err != nil {
			return err
		}
		err = configSet("uploadpack.allowAnySHA1InWant", "true")
	} else {
		if err = configUnsetAll("uploadpack.allowfilter", "true"); err != nil {
			return err
		}
		err = configUnsetAll("uploadpack.allowAnySHA1InWant", "true")
	}

	return err
}

func configSet(key, value string) error {
	stdout, _, err := NewCommand(DefaultContext, "config", "--global", "--get").AddDynamicArguments(key).RunStdString(nil)
	if err != nil && !IsErrorExitCode(err, 1) {
		return fmt.Errorf("failed to get git config %s, err: %w", key, err)
	}

	currValue := strings.TrimSpace(stdout)
	if currValue == value {
		return nil
	}

	_, _, err = NewCommand(DefaultContext, "config", "--global").AddDynamicArguments(key, value).RunStdString(nil)
	if err != nil {
		return fmt.Errorf("failed to set git global config %s, err: %w", key, err)
	}

	return nil
}

func configSetNonExist(key, value string) error {
	_, _, err := NewCommand(DefaultContext, "config", "--global", "--get").AddDynamicArguments(key).RunStdString(nil)
	if err == nil {
		// already exist
		return nil
	}
	if IsErrorExitCode(err, 1) {
		// not exist, set new config
		_, _, err = NewCommand(DefaultContext, "config", "--global").AddDynamicArguments(key, value).RunStdString(nil)
		if err != nil {
			return fmt.Errorf("failed to set git global config %s, err: %w", key, err)
		}
		return nil
	}

	return fmt.Errorf("failed to get git config %s, err: %w", key, err)
}

func configAddNonExist(key, value string) error {
	_, _, err := NewCommand(DefaultContext, "config", "--global", "--get").AddDynamicArguments(key, regexp.QuoteMeta(value)).RunStdString(nil)
	if err == nil {
		// already exist
		return nil
	}
	if IsErrorExitCode(err, 1) {
		// not exist, add new config
		_, _, err = NewCommand(DefaultContext, "config", "--global", "--add").AddDynamicArguments(key, value).RunStdString(nil)
		if err != nil {
			return fmt.Errorf("failed to add git global config %s, err: %w", key, err)
		}
		return nil
	}
	return fmt.Errorf("failed to get git config %s, err: %w", key, err)
}

func configUnsetAll(key, value string) error {
	_, _, err := NewCommand(DefaultContext, "config", "--global", "--get").AddDynamicArguments(key).RunStdString(nil)
	if err == nil {
		// exist, need to remove
		_, _, err = NewCommand(DefaultContext, "config", "--global", "--unset-all").AddDynamicArguments(key, regexp.QuoteMeta(value)).RunStdString(nil)
		if err != nil {
			return fmt.Errorf("failed to unset git global config %s, err: %w", key, err)
		}
		return nil
	}
	if IsErrorExitCode(err, 1) {
		// not exist
		return nil
	}
	return fmt.Errorf("failed to get git config %s, err: %w", key, err)
}

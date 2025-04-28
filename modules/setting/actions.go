// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// Actions settings
var (
	Actions = struct {
		Enabled               bool
		LogStorage            *Storage          // how the created logs should be stored
		LogRetentionDays      int64             `ini:"LOG_RETENTION_DAYS"`
		LogCompression        logCompression    `ini:"LOG_COMPRESSION"`
		ArtifactStorage       *Storage          // how the created artifacts should be stored
		ArtifactRetentionDays int64             `ini:"ARTIFACT_RETENTION_DAYS"`
		DefaultActionsURL     defaultActionsURL `ini:"DEFAULT_ACTIONS_URL"`
		ZombieTaskTimeout     time.Duration     `ini:"ZOMBIE_TASK_TIMEOUT"`
		EndlessTaskTimeout    time.Duration     `ini:"ENDLESS_TASK_TIMEOUT"`
		AbandonedJobTimeout   time.Duration     `ini:"ABANDONED_JOB_TIMEOUT"`
		SkipWorkflowStrings   []string          `ìni:"SKIP_WORKFLOW_STRINGS"`
	}{
		Enabled:             true,
		DefaultActionsURL:   defaultActionsURLGitHub,
		SkipWorkflowStrings: []string{"[skip ci]", "[ci skip]", "[no ci]", "[skip actions]", "[actions skip]"},
	}
)

type defaultActionsURL string

func (url defaultActionsURL) URL() string {
	switch url {
	case defaultActionsURLGitHub:
		return "https://github.com"
	case defaultActionsURLSelf:
		return strings.TrimSuffix(AppURL, "/")
	default:
		// This should never happen, but just in case, use GitHub as fallback
		return "https://github.com"
	}
}

const (
	defaultActionsURLGitHub = "github" // https://github.com
	defaultActionsURLSelf   = "self"   // the root URL of the self-hosted Gitea instance
	// DefaultActionsURL only supports GitHub and the self-hosted Gitea.
	// It's intentionally not supported more, so please be cautious before adding more like "gitea" or "gitlab".
	// If you get some trouble with `uses: username/action_name@version` in your workflow,
	// please consider to use `uses: https://the_url_you_want_to_use/username/action_name@version` instead.
)

type logCompression string

func (c logCompression) IsValid() bool {
	return c.IsNone() || c.IsZstd()
}

func (c logCompression) IsNone() bool {
	return strings.ToLower(string(c)) == "none"
}

func (c logCompression) IsZstd() bool {
	return c == "" || strings.ToLower(string(c)) == "zstd"
}

func loadActionsFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("actions")
	err := sec.MapTo(&Actions)
	if err != nil {
		return fmt.Errorf("failed to map Actions settings: %v", err)
	}

	if urls := string(Actions.DefaultActionsURL); urls != defaultActionsURLGitHub && urls != defaultActionsURLSelf {
		url := strings.Split(urls, ",")[0]
		if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
			log.Error("[actions] DEFAULT_ACTIONS_URL does not support %q as custom URL any longer, fallback to %q",
				urls,
				defaultActionsURLGitHub,
			)
			Actions.DefaultActionsURL = defaultActionsURLGitHub
		} else {
			return fmt.Errorf("unsupported [actions] DEFAULT_ACTIONS_URL: %q", urls)
		}
	}

	// don't support to read configuration from [actions]
	Actions.LogStorage, err = getStorage(rootCfg, "actions_log", "", nil)
	if err != nil {
		return err
	}
	// default to 1 year
	if Actions.LogRetentionDays <= 0 {
		Actions.LogRetentionDays = 365
	}

	actionsSec, _ := rootCfg.GetSection("actions.artifacts")

	Actions.ArtifactStorage, err = getStorage(rootCfg, "actions_artifacts", "", actionsSec)
	if err != nil {
		return err
	}

	// default to 90 days in Github Actions
	if Actions.ArtifactRetentionDays <= 0 {
		Actions.ArtifactRetentionDays = 90
	}

	Actions.ZombieTaskTimeout = sec.Key("ZOMBIE_TASK_TIMEOUT").MustDuration(10 * time.Minute)
	Actions.EndlessTaskTimeout = sec.Key("ENDLESS_TASK_TIMEOUT").MustDuration(3 * time.Hour)
	Actions.AbandonedJobTimeout = sec.Key("ABANDONED_JOB_TIMEOUT").MustDuration(24 * time.Hour)

	if !Actions.LogCompression.IsValid() {
		return fmt.Errorf("invalid [actions] LOG_COMPRESSION: %q", Actions.LogCompression)
	}

	return nil
}

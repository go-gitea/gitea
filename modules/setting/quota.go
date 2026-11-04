// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"math"
	"strings"

	"gitea.dev/modules/util"
)

var Quota = struct {
	DefaultRepoGitLimit int64
	DefaultRepoLFSLimit int64
	DefaultUserLimit    int64
	DefaultOrgLimit     int64
}{-1, -1, -1, -1}

func ParseQuotaLimit(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "-1" {
		return -1, nil
	}
	size, err := util.ParseByteSize(value)
	if err != nil || size > math.MaxInt64 {
		return 0, fmt.Errorf("ParseQuotaLimit: invalid storage limit %q", value)
	}
	return int64(size), nil
}

func loadQuotaFrom(cfg ConfigProvider) error {
	sec := cfg.Section("quota")
	for _, field := range []struct {
		key string
		dst *int64
	}{
		{"DEFAULT_REPO_GIT_LIMIT", &Quota.DefaultRepoGitLimit},
		{"DEFAULT_REPO_LFS_LIMIT", &Quota.DefaultRepoLFSLimit},
		{"DEFAULT_USER_LIMIT", &Quota.DefaultUserLimit},
		{"DEFAULT_ORG_LIMIT", &Quota.DefaultOrgLimit},
	} {
		value, err := ParseQuotaLimit(sec.Key(field.key).MustString("-1"))
		if err != nil {
			return fmt.Errorf("loadQuotaFrom: %s: %w", field.key, err)
		}
		*field.dst = value
	}
	return nil
}

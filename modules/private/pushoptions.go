// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/optional"
)

// GitPushOptions is a wrapper around a map[string]string
type GitPushOptions map[string]string

// GitPushOptions keys
const (
	GitPushOptionRepoPrivate  = "repo.private"
	GitPushOptionRepoTemplate = "repo.template"
	GitPushOptionForcePush    = "force-push"
)

// Bool checks for a key in the map and parses as a boolean
// An option without value is considered true, eg: "-o force-push" or "-o repo.private"
func (g GitPushOptions) Bool(key string) optional.Option[bool] {
	if val, ok := g[key]; ok {
		if val == "" {
			return optional.Some(true)
		}
		if b, err := strconv.ParseBool(val); err == nil {
			return optional.Some(b)
		}
	}
	return optional.None[bool]()
}

// AddFromKeyValue adds a key value pair to the map by "key=value" format or "key" for empty value
func (g GitPushOptions) AddFromKeyValue(line string) {
	kv := strings.SplitN(line, "=", 2)
	if len(kv) == 2 {
		g[kv[0]] = kv[1]
	} else {
		g[kv[0]] = ""
	}
}

// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system

// RuntimeState contains app state for runtime
type RuntimeState struct {
	LastAppPath    string `json:"last_app_path"`
	LastCustomConf string `json:"last_custom_conf"`
}

func (a RuntimeState) StateName() string {
	return "runtime-state"
}

type LastConfig struct {
	ForceUserActivityVisibility string `json:"force_user_activity_visibility"`
}

func (a LastConfig) StateName() string {
	return "last-config"
}

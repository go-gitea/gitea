// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package appstate

// RuntimeState contains app state for runtime, and we can save remote version for update checker here in future
type RuntimeState struct {
	LastAppPath    string `json:"last_app_path"`
	LastCustomConf string `json:"last_custom_conf"`
}

// Name returns the item name
func (a RuntimeState) Name() string {
	return "runtime-state"
}

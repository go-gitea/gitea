// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Global settings
var (
	// RunUser is the OS user that Gitea is running as. ini:"RUN_USER"
	RunUser string
	// RunMode is the running mode of Gitea, it only accepts two values: "dev" and "prod".
	// Non-dev values will be replaced by "prod". ini: "RUN_MODE"
	RunMode string
	// IsProd is true if RunMode is not "dev"
	IsProd bool

	// AppName is the Application name, used in the page title. ini: "APP_NAME"
	AppName string
)

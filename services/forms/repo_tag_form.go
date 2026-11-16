// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import "gitea.dev/modules/web/middleware"

// ProtectTagForm form for changing protected tag settings
type ProtectTagForm struct {
	middleware.FormDefaultValidator
	NamePattern    string `binding:"Required;GlobOrRegexPattern"`
	AllowlistUsers string
	AllowlistTeams string
}

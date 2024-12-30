// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"code.gitea.io/gitea/modules/optional"
)

const (
	AttributeLinguistVendored      = "linguist-vendored"
	AttributeLinguistGenerated     = "linguist-generated"
	AttributeLinguistDocumentation = "linguist-documentation"
	AttributeLinguistDetectable    = "linguist-detectable"
	AttributeLinguistLanguage      = "linguist-language"
	AttributeGitlabLanguage        = "gitlab-language"
)

// true if "set"/"true", false if "unset"/"false", none otherwise
func AttributeToBool(attr map[string]string, name string) optional.Option[bool] {
	switch attr[name] {
	case "set", "true":
		return optional.Some(true)
	case "unset", "false":
		return optional.Some(false)
	}
	return optional.None[bool]()
}

func AttributeToString(attr map[string]string, name string) optional.Option[string] {
	if value, has := attr[name]; has && value != "unspecified" {
		return optional.Some(value)
	}
	return optional.None[string]()
}

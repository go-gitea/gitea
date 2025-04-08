// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attribute

import (
	"strings"

	"code.gitea.io/gitea/modules/optional"
)

type Attribute string

const (
	LinguistVendored      = "linguist-vendored"
	LinguistGenerated     = "linguist-generated"
	LinguistDocumentation = "linguist-documentation"
	LinguistDetectable    = "linguist-detectable"
	LinguistLanguage      = "linguist-language"
	GitlabLanguage        = "gitlab-language"
)

func (a Attribute) ToString() optional.Option[string] {
	if a != "" && a != "unspecified" {
		return optional.Some(string(a))
	}
	return optional.None[string]()
}

// true if "set"/"true", false if "unset"/"false", none otherwise
func (a Attribute) ToBool() optional.Option[bool] {
	switch a {
	case "set", "true":
		return optional.Some(true)
	case "unset", "false":
		return optional.Some(false)
	}
	return optional.None[bool]()
}

type Attributes map[string]Attribute

func (attrs Attributes) Get(name string) Attribute {
	if value, has := attrs[name]; has {
		return value
	}
	return ""
}

func (attrs Attributes) HasVendored() optional.Option[bool] {
	return attrs.Get(LinguistVendored).ToBool()
}

func (attrs Attributes) HasGenerated() optional.Option[bool] {
	return attrs.Get(LinguistGenerated).ToBool()
}

func (attrs Attributes) HasDocumentation() optional.Option[bool] {
	return attrs.Get(LinguistDocumentation).ToBool()
}

func (attrs Attributes) HasDetectable() optional.Option[bool] {
	return attrs.Get(LinguistDetectable).ToBool()
}

func (attrs Attributes) LinguistLanguage() optional.Option[string] {
	return attrs.Get(LinguistLanguage).ToString()
}

func (attrs Attributes) GitlabLanguage() optional.Option[string] {
	attrStr := attrs.Get(GitlabLanguage).ToString()
	if attrStr.Has() {
		raw := attrStr.Value()
		// gitlab-language may have additional parameters after the language
		// ignore them and just use the main language
		// https://docs.gitlab.com/ee/user/project/highlighting.html#override-syntax-highlighting-for-a-file-type
		if idx := strings.IndexByte(raw, '?'); idx >= 0 {
			return optional.Some(raw[:idx])
		}
	}
	return attrStr
}

func (attrs Attributes) Language() optional.Option[string] {
	// prefer linguist-language over gitlab-language
	// if linguist-language is not set, use gitlab-language
	// if both are not set, return none
	language := attrs.LinguistLanguage()
	if language.Value() == "" {
		language = attrs.GitlabLanguage()
	}
	return language
}

// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/microcosm-cc/bluemonday"
)

func (st *Sanitizer) addSanitizerRules(policy *bluemonday.Policy, rules []setting.MarkupSanitizerRule) {
	for _, rule := range rules {
		if rule.AllowDataURIImages {
			policy.AllowDataURIImages()
		}
		if rule.Element != "" {
			if rule.Regexp != "" {
				if !strings.HasPrefix(rule.Regexp, "^") || !strings.HasSuffix(rule.Regexp, "$") {
					panic("Markup sanitizer rule regexp must start with ^ and end with $ to be strict")
				}
				policy.AllowAttrs(rule.AllowAttr).Matching(regexp.MustCompile(rule.Regexp)).OnElements(rule.Element)
			} else {
				policy.AllowAttrs(rule.AllowAttr).OnElements(rule.Element)
			}
		}
	}
}

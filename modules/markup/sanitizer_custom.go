// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"code.gitea.io/gitea/modules/setting"

	"github.com/microcosm-cc/bluemonday"
)

func (st *Sanitizer) addSanitizerRules(policy *bluemonday.Policy, rules []setting.MarkupSanitizerRule) {
	for _, rule := range rules {
		if rule.AllowDataURIImages {
			policy.AllowDataURIImages()
		}
		if rule.Element != "" {
			if rule.Regexp != nil {
				policy.AllowAttrs(rule.AllowAttr).Matching(rule.Regexp).OnElements(rule.Element)
			} else {
				policy.AllowAttrs(rule.AllowAttr).OnElements(rule.Element)
			}
		}
	}
}

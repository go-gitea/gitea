// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// This file is heavily inspired by https://github.com/nicksnyder/go-i18n/tree/main/v2/internal/plural

package plurals

import (
	"testing"
)

func TestRules(t *testing.T) {
	expectedRule := &Rule{}

	testCases := []struct {
		name   string
		rules  Rules
		locale string
		rule   *Rule
	}{
		{
			name: "exact match",
			rules: Rules{"cardinal": map[string]*Rule{
				"en": expectedRule,
				"es": {},
			}},
			locale: "en",
			rule:   expectedRule,
		},
		{
			name: "inexact match",
			rules: Rules{"cardinal": map[string]*Rule{
				"en": expectedRule,
			}},
			locale: "en-US",
			rule:   expectedRule,
		},
		{
			name: "portuguese doesn't match european portuguese",
			rules: Rules{"cardinal": map[string]*Rule{
				"pt-PT": {},
			}},
			locale: "pt",
			rule:   nil,
		},
		{
			name: "european portuguese preferred",
			rules: Rules{"cardinal": map[string]*Rule{
				"pt":    {},
				"pt-PT": expectedRule,
			}},
			locale: "pt-PT",
			rule:   expectedRule,
		},
		{
			name: "zh-Hans",
			rules: Rules{"cardinal": map[string]*Rule{
				"zh": expectedRule,
			}},
			locale: "zh-Hans",
			rule:   expectedRule,
		},
		{
			name: "zh-Hant",
			rules: Rules{"cardinal": map[string]*Rule{
				"zh": expectedRule,
			}},
			locale: "zh-Hant",
			rule:   expectedRule,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if rule := testCase.rules.Rule(testCase.locale); rule != testCase.rule {
				panic(rule)
			}
		})
	}
}

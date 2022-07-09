// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// This file is heavily inspired by https://github.com/nicksnyder/go-i18n/tree/main/v2/internal/plural

package plurals

import (
	"strings"
)

type RuleType string

const (
	Cardinal RuleType = "cardinal"
	Ordinal  RuleType = "ordinal"
)

// Rule defines the CLDR plural rules for a language.
// http://www.unicode.org/cldr/charts/latest/supplemental/language_plural_rules.html
// http://unicode.org/reports/tr35/tr35-numbers.html#Operands
type Rule struct {
	PluralForms    map[Form]struct{}
	PluralFormFunc func(*Operands) Form
}

func addPluralRules(rules *Rules, typ RuleType, ids []string, ps *Rule) {
	for _, id := range ids {
		if id == "root" {
			continue
		}
		switch typ {
		case Cardinal:
			if rules.CardinalMap == nil {
				rules.CardinalMap = map[string]*Rule{}
			}
			rules.CardinalMap[id] = ps
		case Ordinal:
			if rules.OrdinalMap == nil {
				rules.OrdinalMap = map[string]*Rule{}
			}
			rules.OrdinalMap[id] = ps
		default:
			if rules.Others == nil {
				rules.Others = map[RuleType]map[string]*Rule{}
			}
			if rules.Others[typ] == nil {
				rules.Others[typ] = map[string]*Rule{}
			}
			rules.Others[typ][id] = ps
		}
	}
}

func newPluralFormSet(pluralForms ...Form) map[Form]struct{} {
	set := make(map[Form]struct{}, len(pluralForms))
	for _, plural := range pluralForms {
		set[plural] = struct{}{}
	}
	return set
}

type Rules struct {
	CardinalMap map[string]*Rule
	OrdinalMap  map[string]*Rule
	Others      map[RuleType]map[string]*Rule
}

// Rule returns the closest matching plural rule for the language tag
// or nil if no rule could be found.
func (r Rules) Rule(locale string) *Rule {
	for {
		if rule, ok := r.CardinalMap[locale]; ok {
			return rule
		}
		idx := strings.LastIndex(locale, "-")
		if idx < 0 {
			return r.CardinalMap["en"]
		}
		locale = locale[:idx]
	}
}

// Rule returns the closest matching plural rule for the language tag
// or nil if no rule could be found.
func (r Rules) Ordinal(locale string) *Rule {
	for {
		if rule, ok := r.OrdinalMap[locale]; ok {
			return rule
		}
		idx := strings.LastIndex(locale, "-")
		if idx < 0 {
			return r.OrdinalMap["en"]
		}
		locale = locale[:idx]
	}
}

// Rule returns the closest matching plural rule for the language tag
// or nil if no rule could be found.
func (r Rules) RuleByType(typ RuleType, locale string) *Rule {
	switch typ {
	case Cardinal:
		return r.Rule(locale)
	case Ordinal:
		return r.Ordinal(locale)
	}
	for {
		if rule, ok := r.Others[typ][locale]; ok {
			return rule
		}
		idx := strings.LastIndex(locale, "-")
		if idx < 0 {
			return r.Others[typ]["en"]
		}
		locale = locale[:idx]
	}
}

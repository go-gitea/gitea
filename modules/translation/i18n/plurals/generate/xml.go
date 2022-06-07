// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:generate go run main/generate.go -c ../rules_gen.go -t ../rules_gen_test.go plurals.xml ordinals.xml

// This file is heavily inspired by https://github.com/nicksnyder/go-i18n/tree/main/v2/internal/plural

package generate

import (
	"encoding/xml"
	"strings"
)

// SupplementalData is the root of plural.xml
type SupplementalData struct {
	XMLName xml.Name  `xml:"supplementalData"`
	Plurals []Plurals `xml:"plurals"`
}

// SupplementalData is the root of plural.xml
type Plurals struct {
	Type         string        `xml:"type,attr"`
	LocaleGroups []LocaleGroup `xml:"pluralRules"`
}

// LocaleGroup is a group of locales with the same plural rules.
type LocaleGroup struct {
	Locales string `xml:"locales,attr"`
	Rules   []Rule `xml:"pluralRule"`
}

// SplitLocales returns all the locales in the PluralGroup as a slice.
func (lg *LocaleGroup) SplitLocales() []string {
	return strings.Split(lg.Locales, " ")
}

// Rule is a rule for a single plural form.
type Rule struct {
	// Count is one of `zero` | `one` | `two` | `few` | `many` | `other`
	Count string `xml:"count,attr"`
	// Rule looks like:
	// <condition> (@integer <integer-examples> (…)?)? (@decimal <decimal-examples> (…)?)?
	// <condition> ">n % 10 = 1 and n % 100 != 11..19"
	// <integer> "@integer 1, 21, 31, 41, 51, 61, 71, 81, 101, 1001, …"
	// <decimal> "@decimal 1.0, 21.0, 31.0, 41.0, 51.0, 61.0, 71.0, 81.0, 101.0, 1001.0, …"
	Rule string `xml:",innerxml"`
}

// CountTitle returns the title case of the pluralRule's count.
func (r *Rule) CountTitle() string {
	return strings.ToUpper(r.Count[0:1]) + r.Count[1:]
}

// Condition returns the condition where the pluralRule applies.
// These look like "", ">n % 10 = 1 and n % 100 != 11..19" etc.
//
// The conditions themselves have the following syntax.
//
// condition       = and_condition ('or' and_condition)*
// and_condition   = relation ('and' relation)*
//
// Now the next bit needs some adjustment
//
// relation        = is_relation | in_relation | within_relation
// is_relation     = expr 'is' ('not')? value
//     ^------------ This is not present in plurals.xml/ordinals.xml
// in_relation     = expr (('not')? 'in' | '=' | '!=') range_list
//                   ^^^^^^^^^^^^^^^^^^^^^ not in plurals.xml/ordinals.xml
// within_relation = expr ('not')? 'within' range_list
//    ^------------- This is not present in plurals.xml/ordinals.xml
//
// So relation is really:
//
// relation        = expr ('=' | '!=') range_list
//
// expr            = operand (('mod' | '%') value)?
//                             ^^^^^ not in plurals.xml/ordinals.xml
// operand         = 'n' | 'i' | 'f' | 't' | 'v' | 'w' | 'c' | 'e'
//                     not in plurals.xml/ordinals.xml ^^^^^^^^^^^
// range_list      = (range | value) (',' range_list)*
// range           = value'..'value
// value           = digit+
// digit           = [0-9]
func (r *Rule) Condition() string {
	i := strings.Index(r.Rule, "@")
	if i >= 0 {
		return r.Rule[:i]
	}
	return r.Rule
}

// Samples returns the integer and decimal samples for the pluralRule
//
// samples         = ('@integer' sampleList)?
//                   ('@decimal' sampleList)?
// sampleList      = sampleRange (',' sampleRange)* (',' ('…'|'...'))?
// sampleRange     = sampleValue ('~' sampleValue)?
// sampleValue     = value ('.' digit+)? ([ce] digitPos digit+)?
// value           = digit+
// digit           = [0-9]
// 1        = [1-9]
func (r *Rule) Samples() (integer, decimal []string) {
	// First of all remove the ellipses as they're not helpful
	rule := strings.ReplaceAll(r.Rule, ", …", "")

	// Now the we know that @decimal is always after the @integer section
	ruleSplit := strings.SplitN(rule, " @decimal ", 2)
	rule = ruleSplit[0]

	if len(ruleSplit) > 1 {
		decimal = strings.Split(ruleSplit[1], ", ")
	}

	ruleSplit = strings.SplitN(rule, " @integer ", 2)
	if len(ruleSplit) > 1 {
		integer = strings.Split(ruleSplit[1], ", ")
	}

	return integer, decimal
}

// IntegerSamples returns the integer exmaples for the PLuralRule.
func (r *Rule) IntegerSamples() []string {
	integer, _ := r.Samples()
	return integer
}

// DecimalSamples returns the decimal exmaples for the PLuralRule.
func (r *Rule) DecimalSamples() []string {
	_, decimal := r.Samples()
	return decimal
}

// GoCondition returns the converted CLDR plural rules to Go code
func (r *Rule) GoCondition() string {
	return ConditionToGoString(r.Condition())
}

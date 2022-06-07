// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// This file is heavily inspired by https://github.com/nicksnyder/go-i18n/tree/main/v2/internal/plural

package plurals

import (
	"strconv"
	"strings"
	"testing"
)

type pluralFormTest struct {
	num  interface{}
	typ  string
	form Form
}

func runTests(t *testing.T, pluralRuleID, typ string, tests []pluralFormTest) {
	if pluralRuleID == "root" {
		return
	}
	pluralRules := DefaultRules()
	if rule := pluralRules.RuleByType(RuleType(typ), pluralRuleID); rule != nil {
		for _, test := range tests {
			ops, err := NewOperands(test.num)
			if err != nil {
				t.Errorf("%s: NewOperands(%v) errored with %s", pluralRuleID, test.num, err)
				break
			}
			if pluralForm := rule.PluralFormFunc(ops); pluralForm != test.form {
				t.Errorf("%s:%v: PluralFormFunc(%#v) returned %q, %v; expected %q", pluralRuleID, test.num, ops, pluralForm, err, test.form)
			}
		}
	} else {
		t.Errorf("could not find plural rule for locale %s", pluralRuleID)
	}
}

func appendIntegerTests(tests []pluralFormTest, typ string, form Form, examples []string) []pluralFormTest {
	for _, ex := range expandExamples(examples) {
		var i int64
		if strings.Count(ex, "c") == 1 || strings.Count(ex, "e") == 1 {
			ex = strings.Replace(ex, "e", "c", 1)
			// Now the problem is s could be in [1-9](.[0-9]+)?e[1-9][0-9]*
			// We need to determine how many numbers after the decimal place remain.
			if parts := strings.SplitN(ex, "c", 2); len(parts) == 2 {
				if idx := strings.Index(parts[0], "."); idx >= 0 {
					numberOfDecimalsPreExp := len(parts[0]) - idx - 1
					exp, err := strconv.Atoi(parts[1])
					if err != nil {
						panic(err)
					}
					if exp >= numberOfDecimalsPreExp {
						ex = parts[0][:idx] + parts[0][idx+1:]
						exp -= numberOfDecimalsPreExp
						ex += strings.Repeat("0", exp)
					} else {
						ex = parts[0][:idx] + parts[0][idx+1:len(parts[0])+exp-numberOfDecimalsPreExp] + "." + parts[0][len(parts[0])+exp-numberOfDecimalsPreExp:]
					}
				} else {
					exp, err := strconv.Atoi(parts[1])
					if err != nil {
						panic(err)
					}
					ex = parts[0] + strings.Repeat("0", exp)
				}
			}
		}

		var err error
		i, err = strconv.ParseInt(ex, 10, 64)
		if err != nil {
			panic(err)
		}
		tests = append(tests, pluralFormTest{ex, typ, form}, pluralFormTest{i, typ, form})
	}
	return tests
}

func appendDecimalTests(tests []pluralFormTest, typ string, form Form, examples []string) []pluralFormTest {
	for _, ex := range expandExamples(examples) {
		ex = strings.Replace(ex, "c", "e", 1)
		tests = append(tests, pluralFormTest{ex, typ, form})
	}
	return tests
}

func expandExamples(examples []string) []string {
	var expanded []string
	for _, ex := range examples {
		ex = strings.Replace(ex, "c", "e", 1)
		if parts := strings.Split(ex, "~"); len(parts) == 2 {
			for ex := parts[0]; ; ex = increment(ex) {
				expanded = append(expanded, ex)
				if ex == parts[1] {
					break
				}
			}
		} else {
			expanded = append(expanded, ex)
		}
	}
	return expanded
}

func increment(dec string) string {
	runes := []rune(dec)
	carry := true
	for i := len(runes) - 1; carry && i >= 0; i-- {
		switch runes[i] {
		case '.':
			continue
		case '9':
			runes[i] = '0'
		default:
			runes[i]++
			carry = false
		}
	}
	if carry {
		runes = append([]rune{'1'}, runes...)
	}
	return string(runes)
}

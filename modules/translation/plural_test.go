// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package translation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message/catalog"
)

func matchInt(l language.Tag, v any) plural.Form {
	digits := []byte(fmt.Sprint(v))
	for i := range digits {
		digits[i] -= '0'
	}
	return plural.Cardinal.MatchDigits(l, digits, len(digits), 0)
}

var formsAll = []plural.Form{
	plural.Zero,
	plural.One,
	plural.Two,
	plural.Few,
	plural.Many,
	plural.Other,
}

func supportedForms(l language.Tag) (forms []plural.Form) {
	cat := catalog.NewBuilder()
	for _, form := range formsAll {
		err := cat.Set(l, "%d test", plural.Selectf(1, "%d", form, "%d test"))
		if err == nil {
			forms = append(forms, form)
		}
	}
	return forms
}

func TrPlural(langTag language.Tag, format string, args ...any) string {
	// HINT: this is for demo purpose only, not optimized
	form := plural.Other
	forms := supportedForms(langTag)
	for _, arg := range args {
		switch arg.(type) {
		case int, int64:
			form = matchInt(langTag, arg)
		}
	}
	p1 := strings.Index(format, "$[")
	p2 := strings.Index(format, "]")
	if p1 != -1 && p2 != -1 && p1 < p2 {
		words := strings.Split(format[p1+2:p2], ",")
		word := words[len(words)-1]
		for i := range words {
			if forms[i] == form {
				word = words[i]
			}
		}
		format = format[:p1] + strings.TrimSpace(word) + format[p2+1:]
	}
	return fmt.Sprintf(format, args...)
}

func TestPlural(t *testing.T) {
	msg := TrPlural(language.English, "%d $[one, other]", 0)
	assert.Equal(t, "0 other", msg)
	msg = TrPlural(language.English, "%d $[one, other]", 1)
	assert.Equal(t, "1 one", msg)
	msg = TrPlural(language.English, "%d $[one, other]", 2)
	assert.Equal(t, "2 other", msg)

	msg = TrPlural(language.Latvian, "%d $[zero, one, other]", 0)
	assert.Equal(t, "0 zero", msg)
	msg = TrPlural(language.Latvian, "%d $[zero, one, other]", 1)
	assert.Equal(t, "1 one", msg)
	msg = TrPlural(language.Latvian, "%d $[zero, one, other]", 2)
	assert.Equal(t, "2 other", msg)

	msg = TrPlural(language.Arabic, "%d $[zero, one, two, few, many, other]", 0)
	assert.Equal(t, "0 zero", msg)
	msg = TrPlural(language.Arabic, "%d $[zero, one, two, few, many, other]", 1)
	assert.Equal(t, "1 one", msg)
	msg = TrPlural(language.Arabic, "%d $[zero, one, two, few, many, other]", 2)
	assert.Equal(t, "2 two", msg)
	msg = TrPlural(language.Arabic, "%d $[zero, one, two, few, many, other]", 3)
	assert.Equal(t, "3 few", msg)
	msg = TrPlural(language.Arabic, "%d $[zero, one, two, few, many, other]", 11)
	assert.Equal(t, "11 many", msg)
	msg = TrPlural(language.Arabic, "%d $[zero, one, two, few, many, other]", 100)
	assert.Equal(t, "100 other", msg)
}

// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package analyze

import (
	"regexp"
	"sort"
	"strings"

	"github.com/go-enry/go-enry/v2/data"
)

var isVendorRegExp *regexp.Regexp

func init() {
	matchers := data.VendorMatchers

	caretStrings := make([]string, 0, 10)
	caretShareStrings := make([]string, 0, 10)

	matcherStrings := make([]string, 0, len(matchers))
	for _, matcher := range matchers {
		str := matcher.String()
		if str[0] == '^' {
			caretStrings = append(caretStrings, str[1:])
		} else if str[0:5] == "(^|/)" {
			caretShareStrings = append(caretShareStrings, str[5:])
		} else {
			matcherStrings = append(matcherStrings, str)
		}
	}

	sort.Strings(caretShareStrings)
	sort.Strings(caretStrings)
	sort.Strings(matcherStrings)

	sb := &strings.Builder{}
	sb.WriteString("(?:^(?:")
	sb.WriteString(caretStrings[0])
	for _, matcher := range caretStrings[1:] {
		sb.WriteString(")|(?:")
		sb.WriteString(matcher)
	}
	sb.WriteString("))")
	sb.WriteString("|")
	sb.WriteString("(?:(?:^|/)(?:")
	sb.WriteString(caretShareStrings[0])
	for _, matcher := range caretShareStrings[1:] {
		sb.WriteString(")|(?:")
		sb.WriteString(matcher)
	}
	sb.WriteString("))")
	sb.WriteString("|")
	sb.WriteString("(?:")
	sb.WriteString(matcherStrings[0])
	for _, matcher := range matcherStrings[1:] {
		sb.WriteString(")|(?:")
		sb.WriteString(matcher)
	}
	sb.WriteString(")")
	combined := sb.String()
	isVendorRegExp = regexp.MustCompile(combined)
}

// IsVendor returns whether or not path is a vendor path.
func IsVendor(path string) bool {
	return isVendorRegExp.MatchString(path)
}

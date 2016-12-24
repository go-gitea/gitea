// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package stringutil

import (
	"bytes"
	"strings"
)

// See: https://dev.mysql.com/doc/refman/5.7/en/string-literals.html#character-escape-sequences
const validEscapeChars = `0'"bntrz\\%_`

// RemoveUselessBackslash removes backslashs which could be ignored in the string literal.
// See: https://dev.mysql.com/doc/refman/5.7/en/string-literals.html
// " Each of these sequences begins with a backslash (“\”), known as the escape character.
// MySQL recognizes the escape sequences shown in Table 9.1, “Special Character Escape Sequences”.
// For all other escape sequences, backslash is ignored. That is, the escaped character is
// interpreted as if it was not escaped. For example, “\x” is just “x”. These sequences are case sensitive.
// For example, “\b” is interpreted as a backspace, but “\B” is interpreted as “B”."
func RemoveUselessBackslash(s string) string {
	var (
		buf bytes.Buffer
		i   = 0
	)
	for i < len(s)-1 {
		if s[i] != '\\' {
			buf.WriteByte(s[i])
			i++
			continue
		}
		next := s[i+1]
		if strings.IndexByte(validEscapeChars, next) != -1 {
			buf.WriteByte(s[i])
		}
		buf.WriteByte(next)
		i += 2
	}
	if i == len(s)-1 {
		buf.WriteByte(s[i])
	}
	return buf.String()
}

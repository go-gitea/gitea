// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schemas

import (
	"strings"
)

// Quoter represents a quoter to the SQL table name and column name
type Quoter struct {
	Prefix     byte
	Suffix     byte
	IsReserved func(string) bool
}

var (
	// AlwaysFalseReverse always think it's not a reverse word
	AlwaysNoReserve = func(string) bool { return false }

	// AlwaysReverse always reverse the word
	AlwaysReserve = func(string) bool { return true }

	// CommanQuoteMark represnets the common quote mark
	CommanQuoteMark byte = '`'

	// CommonQuoter represetns a common quoter
	CommonQuoter = Quoter{CommanQuoteMark, CommanQuoteMark, AlwaysReserve}
)

func (q Quoter) IsEmpty() bool {
	return q.Prefix == 0 && q.Suffix == 0
}

func (q Quoter) Quote(s string) string {
	var buf strings.Builder
	q.QuoteTo(&buf, s)
	return buf.String()
}

// Trim removes quotes from s
func (q Quoter) Trim(s string) string {
	if len(s) < 2 {
		return s
	}

	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case i == 0 && s[i] == q.Prefix:
		case i == len(s)-1 && s[i] == q.Suffix:
		case s[i] == q.Suffix && s[i+1] == '.':
		case s[i] == q.Prefix && s[i-1] == '.':
		default:
			buf.WriteByte(s[i])
		}
	}
	return buf.String()
}

func (q Quoter) Join(a []string, sep string) string {
	var b strings.Builder
	q.JoinWrite(&b, a, sep)
	return b.String()
}

func (q Quoter) JoinWrite(b *strings.Builder, a []string, sep string) error {
	if len(a) == 0 {
		return nil
	}

	n := len(sep) * (len(a) - 1)
	for i := 0; i < len(a); i++ {
		n += len(a[i])
	}

	b.Grow(n)
	for i, s := range a {
		if i > 0 {
			if _, err := b.WriteString(sep); err != nil {
				return err
			}
		}
		if s != "*" {
			q.QuoteTo(b, strings.TrimSpace(s))
		}
	}
	return nil
}

func findWord(v string, start int) int {
	for j := start; j < len(v); j++ {
		switch v[j] {
		case '.', ' ':
			return j
		}
	}
	return len(v)
}

func findStart(value string, start int) int {
	if value[start] == '.' {
		return start + 1
	}
	if value[start] != ' ' {
		return start
	}

	var k = -1
	for j := start; j < len(value); j++ {
		if value[j] != ' ' {
			k = j
			break
		}
	}
	if k == -1 {
		return len(value)
	}

	if (value[k] == 'A' || value[k] == 'a') && (value[k+1] == 'S' || value[k+1] == 's') {
		k = k + 2
	}

	for j := k; j < len(value); j++ {
		if value[j] != ' ' {
			return j
		}
	}
	return len(value)
}

func (q Quoter) quoteWordTo(buf *strings.Builder, word string) error {
	var realWord = word
	if (word[0] == CommanQuoteMark && word[len(word)-1] == CommanQuoteMark) ||
		(word[0] == q.Prefix && word[len(word)-1] == q.Suffix) {
		realWord = word[1 : len(word)-1]
	}

	if q.IsEmpty() {
		_, err := buf.WriteString(realWord)
		return err
	}

	isReserved := q.IsReserved(realWord)
	if isReserved {
		if err := buf.WriteByte(q.Prefix); err != nil {
			return err
		}
	}
	if _, err := buf.WriteString(realWord); err != nil {
		return err
	}
	if isReserved {
		return buf.WriteByte(q.Suffix)
	}

	return nil
}

// QuoteTo quotes the table or column names. i.e. if the quotes are [ and ]
//   name -> [name]
//   `name` -> [name]
//   [name] -> [name]
//   schema.name -> [schema].[name]
//   `schema`.`name` -> [schema].[name]
//   `schema`.name -> [schema].[name]
//   schema.`name` -> [schema].[name]
//   [schema].name -> [schema].[name]
//   schema.[name] -> [schema].[name]
//   name AS a  ->  [name] AS a
//   schema.name AS a  ->  [schema].[name] AS a
func (q Quoter) QuoteTo(buf *strings.Builder, value string) error {
	var i int
	for i < len(value) {
		start := findStart(value, i)
		if start > i {
			if _, err := buf.WriteString(value[i:start]); err != nil {
				return err
			}
		}
		if start == len(value) {
			return nil
		}

		var nextEnd = findWord(value, start)
		if err := q.quoteWordTo(buf, value[start:nextEnd]); err != nil {
			return err
		}
		i = nextEnd
	}
	return nil
}

// Strings quotes a slice of string
func (q Quoter) Strings(s []string) []string {
	var res = make([]string, 0, len(s))
	for _, a := range s {
		res = append(res, q.Quote(a))
	}
	return res
}

// Replace replaces common quote(`) as the quotes on the sql
func (q Quoter) Replace(sql string) string {
	if q.IsEmpty() {
		return sql
	}

	var buf strings.Builder
	buf.Grow(len(sql))

	var beginSingleQuote bool
	for i := 0; i < len(sql); i++ {
		if !beginSingleQuote && sql[i] == CommanQuoteMark {
			var j = i + 1
			for ; j < len(sql); j++ {
				if sql[j] == CommanQuoteMark {
					break
				}
			}
			word := sql[i+1 : j]
			isReserved := q.IsReserved(word)
			if isReserved {
				buf.WriteByte(q.Prefix)
			}
			buf.WriteString(word)
			if isReserved {
				buf.WriteByte(q.Suffix)
			}
			i = j
		} else {
			if sql[i] == '\'' {
				beginSingleQuote = !beginSingleQuote
			}
			buf.WriteByte(sql[i])
		}
	}
	return buf.String()
}

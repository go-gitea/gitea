// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package strings

import "testing"

// Test case for any function which accepts and returns a single string.
type StringTest struct {
	in, out string
}

var upperTests = []StringTest{
	{"", ""},
	{"ONLYUPPER", "ONLYUPPER"},
	{"abc", "ABC"},
	{"AbC123", "ABC123"},
	{"azAZ09_", "AZAZ09_"},
	{"longStrinGwitHmixofsmaLLandcAps", "LONGSTRINGWITHMIXOFSMALLANDCAPS"},
	{"long\u0250string\u0250with\u0250nonascii\u2C6Fchars", "LONG\u0250STRING\u0250WITH\u0250NONASCII\u2C6FCHARS"},
	{"\u0250\u0250\u0250\u0250\u0250", "\u0250\u0250\u0250\u0250\u0250"},
	{"a\u0080\U0010FFFF", "A\u0080\U0010FFFF"},
	{"lél", "LéL"},
}

func TestToASCIIUpper(t *testing.T) {
	for _, tc := range upperTests {
		actual := ToASCIIUpper(tc.in)
		if actual != tc.out {
			t.Errorf("ToASCIIUpper(%q) = %q; want %q", tc.in, actual, tc.out)
		}
	}
}

func BenchmarkToUpper(b *testing.B) {
	for _, tc := range upperTests {
		b.Run(tc.in, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				actual := ToASCIIUpper(tc.in)
				if actual != tc.out {
					b.Errorf("ToUpper(%q) = %q; want %q", tc.in, actual, tc.out)
				}
			}
		})
	}
}

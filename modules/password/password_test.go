// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package password

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComplexity_IsComplexEnough(t *testing.T) {
	matchComplexityOnce.Do(func() {})

	testlist := []struct {
		complexity  []string
		truevalues  []string
		falsevalues []string
	}{
		{[]string{"off"}, []string{"1", "-", "a", "A", "ñ", "日本語"}, []string{}},
		{[]string{"lower"}, []string{"abc", "abc!"}, []string{"ABC", "123", "=!$", ""}},
		{[]string{"upper"}, []string{"ABC"}, []string{"abc", "123", "=!$", "abc!", ""}},
		{[]string{"digit"}, []string{"123"}, []string{"abc", "ABC", "=!$", "abc!", ""}},
		{[]string{"spec"}, []string{"=!$", "abc!"}, []string{"abc", "ABC", "123", ""}},
		{[]string{"off"}, []string{"abc", "ABC", "123", "=!$", "abc!", ""}, nil},
		{[]string{"lower", "spec"}, []string{"abc!"}, []string{"abc", "ABC", "123", "=!$", "abcABC123", ""}},
		{[]string{"lower", "upper", "digit"}, []string{"abcABC123"}, []string{"abc", "ABC", "123", "=!$", "abc!", ""}},
		{[]string{""}, []string{"abC=1", "abc!9D"}, []string{"ABC", "123", "=!$", ""}},
	}

	for _, test := range testlist {
		testComplextity(test.complexity)
		for _, val := range test.truevalues {
			assert.True(t, IsComplexEnough(val))
		}
		for _, val := range test.falsevalues {
			assert.False(t, IsComplexEnough(val))
		}
	}

	// Remove settings for other tests
	testComplextity([]string{"off"})
}

func TestComplexity_Generate(t *testing.T) {
	matchComplexityOnce.Do(func() {})

	const maxCount = 50
	const pwdLen = 50

	test := func(t *testing.T, modes []string) {
		testComplextity(modes)
		for i := 0; i < maxCount; i++ {
			pwd, err := Generate(pwdLen)
			assert.NoError(t, err)
			assert.Equal(t, pwdLen, len(pwd))
			assert.True(t, IsComplexEnough(pwd), "Failed complexities with modes %+v for generated: %s", modes, pwd)
		}
	}

	test(t, []string{"lower"})
	test(t, []string{"upper"})
	test(t, []string{"lower", "upper", "spec"})
	test(t, []string{"off"})
	test(t, []string{""})

	// Remove settings for other tests
	testComplextity([]string{"off"})
}

func testComplextity(values []string) {
	// Cleanup previous values
	validChars = ""
	requiredList = make([]complexity, 0, len(values))
	setupComplexity(values)
}

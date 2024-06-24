// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"regexp"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/optional"

	"github.com/stretchr/testify/assert"
)

func TestURLJoin(t *testing.T) {
	type test struct {
		Expected string
		Base     string
		Elements []string
	}
	newTest := func(expected, base string, elements ...string) test {
		return test{Expected: expected, Base: base, Elements: elements}
	}
	for _, test := range []test{
		newTest("https://try.gitea.io/a/b/c",
			"https://try.gitea.io", "a/b", "c"),
		newTest("https://try.gitea.io/a/b/c",
			"https://try.gitea.io/", "/a/b/", "/c/"),
		newTest("https://try.gitea.io/a/c",
			"https://try.gitea.io/", "/a/./b/", "../c/"),
		newTest("a/b/c",
			"a", "b/c/"),
		newTest("a/b/d",
			"a/", "b/c/", "/../d/"),
		newTest("https://try.gitea.io/a/b/c#d",
			"https://try.gitea.io", "a/b", "c#d"),
		newTest("/a/b/d",
			"/a/", "b/c/", "/../d/"),
		newTest("/a/b/c",
			"/a", "b/c/"),
		newTest("/a/b/c#hash",
			"/a", "b/c#hash"),
	} {
		assert.Equal(t, test.Expected, URLJoin(test.Base, test.Elements...))
	}
}

func TestIsEmptyString(t *testing.T) {
	cases := []struct {
		s        string
		expected bool
	}{
		{"", true},
		{" ", true},
		{"   ", true},
		{"  a", false},
	}

	for _, v := range cases {
		assert.Equal(t, v.expected, IsEmptyString(v.s))
	}
}

func Test_NormalizeEOL(t *testing.T) {
	data1 := []string{
		"",
		"This text starts with empty lines",
		"another",
		"",
		"",
		"",
		"Some other empty lines in the middle",
		"more.",
		"And more.",
		"Ends with empty lines too.",
		"",
		"",
		"",
	}

	data2 := []string{
		"This text does not start with empty lines",
		"another",
		"",
		"",
		"",
		"Some other empty lines in the middle",
		"more.",
		"And more.",
		"Ends without EOLtoo.",
	}

	buildEOLData := func(data []string, eol string) []byte {
		return []byte(strings.Join(data, eol))
	}

	dos := buildEOLData(data1, "\r\n")
	unix := buildEOLData(data1, "\n")
	mac := buildEOLData(data1, "\r")

	assert.Equal(t, unix, NormalizeEOL(dos))
	assert.Equal(t, unix, NormalizeEOL(mac))
	assert.Equal(t, unix, NormalizeEOL(unix))

	dos = buildEOLData(data2, "\r\n")
	unix = buildEOLData(data2, "\n")
	mac = buildEOLData(data2, "\r")

	assert.Equal(t, unix, NormalizeEOL(dos))
	assert.Equal(t, unix, NormalizeEOL(mac))
	assert.Equal(t, unix, NormalizeEOL(unix))

	assert.Equal(t, []byte("one liner"), NormalizeEOL([]byte("one liner")))
	assert.Equal(t, []byte("\n"), NormalizeEOL([]byte("\n")))
	assert.Equal(t, []byte("\ntwo liner"), NormalizeEOL([]byte("\ntwo liner")))
	assert.Equal(t, []byte("two liner\n"), NormalizeEOL([]byte("two liner\n")))
	assert.Equal(t, []byte{}, NormalizeEOL([]byte{}))

	assert.Equal(t, []byte("mix\nand\nmatch\n."), NormalizeEOL([]byte("mix\r\nand\rmatch\n.")))
}

func Test_RandomInt(t *testing.T) {
	randInt, err := CryptoRandomInt(255)
	assert.True(t, randInt >= 0)
	assert.True(t, randInt <= 255)
	assert.NoError(t, err)
}

func Test_RandomString(t *testing.T) {
	str1, err := CryptoRandomString(32)
	assert.NoError(t, err)
	matches, err := regexp.MatchString(`^[a-zA-Z0-9]{32}$`, str1)
	assert.NoError(t, err)
	assert.True(t, matches)

	str2, err := CryptoRandomString(32)
	assert.NoError(t, err)
	matches, err = regexp.MatchString(`^[a-zA-Z0-9]{32}$`, str1)
	assert.NoError(t, err)
	assert.True(t, matches)

	assert.NotEqual(t, str1, str2)

	str3, err := CryptoRandomString(256)
	assert.NoError(t, err)
	matches, err = regexp.MatchString(`^[a-zA-Z0-9]{256}$`, str3)
	assert.NoError(t, err)
	assert.True(t, matches)

	str4, err := CryptoRandomString(256)
	assert.NoError(t, err)
	matches, err = regexp.MatchString(`^[a-zA-Z0-9]{256}$`, str4)
	assert.NoError(t, err)
	assert.True(t, matches)

	assert.NotEqual(t, str3, str4)
}

func Test_RandomBytes(t *testing.T) {
	bytes1, err := CryptoRandomBytes(32)
	assert.NoError(t, err)

	bytes2, err := CryptoRandomBytes(32)
	assert.NoError(t, err)

	assert.NotEqual(t, bytes1, bytes2)

	bytes3, err := CryptoRandomBytes(256)
	assert.NoError(t, err)

	bytes4, err := CryptoRandomBytes(256)
	assert.NoError(t, err)

	assert.NotEqual(t, bytes3, bytes4)
}

func TestOptionalBoolParse(t *testing.T) {
	assert.Equal(t, optional.None[bool](), OptionalBoolParse(""))
	assert.Equal(t, optional.None[bool](), OptionalBoolParse("x"))

	assert.Equal(t, optional.Some(false), OptionalBoolParse("0"))
	assert.Equal(t, optional.Some(false), OptionalBoolParse("f"))
	assert.Equal(t, optional.Some(false), OptionalBoolParse("False"))

	assert.Equal(t, optional.Some(true), OptionalBoolParse("1"))
	assert.Equal(t, optional.Some(true), OptionalBoolParse("t"))
	assert.Equal(t, optional.Some(true), OptionalBoolParse("True"))
}

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

func TestToUpperASCII(t *testing.T) {
	for _, tc := range upperTests {
		assert.Equal(t, ToUpperASCII(tc.in), tc.out)
	}
}

func BenchmarkToUpper(b *testing.B) {
	for _, tc := range upperTests {
		b.Run(tc.in, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ToUpperASCII(tc.in)
			}
		})
	}
}

func TestToTitleCase(t *testing.T) {
	assert.Equal(t, ToTitleCase(`foo bar baz`), `Foo Bar Baz`)
	assert.Equal(t, ToTitleCase(`FOO BAR BAZ`), `Foo Bar Baz`)
}

func TestToPointer(t *testing.T) {
	assert.Equal(t, "abc", *ToPointer("abc"))
	assert.Equal(t, 123, *ToPointer(123))
	abc := "abc"
	assert.False(t, &abc == ToPointer(abc))
	val123 := 123
	assert.False(t, &val123 == ToPointer(val123))
}

func TestReserveLineBreakForTextarea(t *testing.T) {
	assert.Equal(t, ReserveLineBreakForTextarea("test\r\ndata"), "test\ndata")
	assert.Equal(t, ReserveLineBreakForTextarea("test\r\ndata\r\n"), "test\ndata\n")
}

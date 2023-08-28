// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURLJoin(t *testing.T) {
	cases := []struct {
		Base     string
		Elems    []string
		Expected string
	}{
		{
			Base:     "https://try.gitea.io",
			Elems:    []string{"a/b", "c"},
			Expected: "https://try.gitea.io/a/b/c",
		},
		{
			Base:     "https://try.gitea.io/",
			Elems:    []string{"a/b", "c"},
			Expected: "https://try.gitea.io/a/b/c",
		},
		{
			Base:     "https://try.gitea.io/",
			Elems:    []string{"/a/b/", "/c/"},
			Expected: "https://try.gitea.io/a/b/c",
		},
		{
			Base:     "https://try.gitea.io/",
			Elems:    []string{"/a/./b/", "../c/"},
			Expected: "https://try.gitea.io/a/c",
		},
		{
			Base:     "a",
			Elems:    []string{"b/c/"},
			Expected: "a/b/c",
		},
		{
			Base:     "a/",
			Elems:    []string{"b/c/", "/../d/"},
			Expected: "a/b/d",
		},
		{
			Base:     "https://try.gitea.io",
			Elems:    []string{"a/b", "c#d"},
			Expected: "https://try.gitea.io/a/b/c#d",
		},
		{
			Base:     "/a/",
			Elems:    []string{"b/c/", "/../d/"},
			Expected: "/a/b/d",
		},
		{
			Base:     "/a",
			Elems:    []string{"b/c/"},
			Expected: "/a/b/c",
		},
		{
			Base:     "/a",
			Elems:    []string{"b/c#hash"},
			Expected: "/a/b/c#hash",
		},
		{
			Base:     "\x7f", // invalid url
			Expected: "",
		},
		{
			Base:     "path",
			Expected: "path/",
		},
		{
			Base:     "/path",
			Expected: "/path/",
		},
		{
			Base:     "path/",
			Expected: "path/",
		},
		{
			Base:     "/path/",
			Expected: "/path/",
		},
		{
			Base:     "path",
			Elems:    []string{"sub"},
			Expected: "path/sub",
		},
		{
			Base:     "/path",
			Elems:    []string{"sub"},
			Expected: "/path/sub",
		},
		{
			Base:     "https://gitea.com",
			Expected: "https://gitea.com/",
		},
		{
			Base:     "https://gitea.com/",
			Expected: "https://gitea.com/",
		},
		{
			Base:     "https://gitea.com",
			Elems:    []string{"sub/path"},
			Expected: "https://gitea.com/sub/path",
		},
		{
			Base:     "https://gitea.com/",
			Elems:    []string{"sub", "path"},
			Expected: "https://gitea.com/sub/path",
		},
		{
			Base:     "https://gitea.com/",
			Elems:    []string{"sub", "..", "path"},
			Expected: "https://gitea.com/path",
		},
		{
			Base:     "https://gitea.com/",
			Elems:    []string{"sub/..", "path"},
			Expected: "https://gitea.com/path",
		},
		{
			Base:     "https://gitea.com/",
			Elems:    []string{"sub", "../path"},
			Expected: "https://gitea.com/path",
		},
		{
			Base:     "https://gitea.com/",
			Elems:    []string{"sub/../path"},
			Expected: "https://gitea.com/path",
		},
		{
			Base:     "https://gitea.com/",
			Elems:    []string{"sub", ".", "path"},
			Expected: "https://gitea.com/sub/path",
		},
		{
			Base:     "https://gitea.com/",
			Elems:    []string{"sub", "/", "path"},
			Expected: "https://gitea.com/sub/path",
		},
		{ // https://github.com/go-gitea/gitea/issues/25632
			Base:     "/owner/repo/media/branch/main",
			Elems:    []string{"/../other/image.png"},
			Expected: "/owner/repo/media/branch/other/image.png",
		},
	}

	for i, c := range cases {
		assert.Equal(t, c.Expected, URLJoin(c.Base, c.Elems...), "Unexpected result in test case %v", i)
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
	int, err := CryptoRandomInt(255)
	assert.True(t, int >= 0)
	assert.True(t, int <= 255)
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

func Test_OptionalBool(t *testing.T) {
	assert.Equal(t, OptionalBoolNone, OptionalBoolParse(""))
	assert.Equal(t, OptionalBoolNone, OptionalBoolParse("x"))

	assert.Equal(t, OptionalBoolFalse, OptionalBoolParse("0"))
	assert.Equal(t, OptionalBoolFalse, OptionalBoolParse("f"))
	assert.Equal(t, OptionalBoolFalse, OptionalBoolParse("False"))

	assert.Equal(t, OptionalBoolTrue, OptionalBoolParse("1"))
	assert.Equal(t, OptionalBoolTrue, OptionalBoolParse("t"))
	assert.Equal(t, OptionalBoolTrue, OptionalBoolParse("True"))
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

func TestBase64FixedDecode(t *testing.T) {
	_, err := Base64FixedDecode(base64.RawURLEncoding, []byte("abcd"), 32)
	assert.ErrorContains(t, err, "invalid base64 decoded length")
	_, err = Base64FixedDecode(base64.RawURLEncoding, []byte(strings.Repeat("a", 64)), 32)
	assert.ErrorContains(t, err, "invalid base64 decoded length")

	str32 := strings.Repeat("x", 32)
	encoded32 := base64.RawURLEncoding.EncodeToString([]byte(str32))
	decoded32, err := Base64FixedDecode(base64.RawURLEncoding, []byte(encoded32), 32)
	assert.NoError(t, err)
	assert.Equal(t, str32, string(decoded32))
}

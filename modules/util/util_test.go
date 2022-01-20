// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"regexp"
	"strings"
	"testing"

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
	int, err := RandomInt(255)
	assert.True(t, int >= 0)
	assert.True(t, int <= 255)
	assert.NoError(t, err)
}

func Test_RandomString(t *testing.T) {
	str1, err := RandomString(32)
	assert.NoError(t, err)
	matches, err := regexp.MatchString(`^[a-zA-Z0-9]{32}$`, str1)
	assert.NoError(t, err)
	assert.True(t, matches)

	str2, err := RandomString(32)
	assert.NoError(t, err)
	matches, err = regexp.MatchString(`^[a-zA-Z0-9]{32}$`, str1)
	assert.NoError(t, err)
	assert.True(t, matches)

	assert.NotEqual(t, str1, str2)

	str3, err := RandomString(256)
	assert.NoError(t, err)
	matches, err = regexp.MatchString(`^[a-zA-Z0-9]{256}$`, str3)
	assert.NoError(t, err)
	assert.True(t, matches)

	str4, err := RandomString(256)
	assert.NoError(t, err)
	matches, err = regexp.MatchString(`^[a-zA-Z0-9]{256}$`, str4)
	assert.NoError(t, err)
	assert.True(t, matches)

	assert.NotEqual(t, str3, str4)
}

func Test_RandomBytes(t *testing.T) {
	bytes1, err := RandomBytes(32)
	assert.NoError(t, err)

	bytes2, err := RandomBytes(32)
	assert.NoError(t, err)

	assert.NotEqual(t, bytes1, bytes2)

	bytes3, err := RandomBytes(256)
	assert.NoError(t, err)

	bytes4, err := RandomBytes(256)
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

// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

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

func TestIsExternalURL(t *testing.T) {
	setting.Domain = "try.gitea.io"
	type test struct {
		Expected bool
		RawURL   string
	}
	newTest := func(expected bool, rawURL string) test {
		return test{Expected: expected, RawURL: rawURL}
	}
	for _, test := range []test{
		newTest(false,
			"https://try.gitea.io"),
		newTest(true,
			"https://example.com/"),
		newTest(true,
			"//example.com"),
		newTest(true,
			"http://example.com"),
		newTest(false,
			"a/"),
		newTest(false,
			"https://try.gitea.io/test?param=false"),
		newTest(false,
			"test?param=false"),
		newTest(false,
			"//try.gitea.io/test?param=false"),
		newTest(false,
			"/hey/hey/hey#3244"),
	} {
		assert.Equal(t, test.Expected, IsExternalURL(test.RawURL))
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

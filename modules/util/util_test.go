// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
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

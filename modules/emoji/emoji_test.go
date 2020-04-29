// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2015 Kenneth Shaw
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package emoji

import (
	"reflect"
	"testing"
)

func TestDumpInfo(t *testing.T) {
	t.Logf("codes: %d", len(codeMap))
	t.Logf("aliases: %d", len(aliasMap))
}

func TestLookup(t *testing.T) {
	a := FromCode("\U0001f37a")
	b := FromCode("🍺")
	c := FromAlias(":beer:")
	d := FromAlias("beer")

	if !reflect.DeepEqual(a, b) {
		t.Errorf("a and b should equal")
	}
	if !reflect.DeepEqual(b, c) {
		t.Errorf("b and c should equal")
	}
	if !reflect.DeepEqual(c, d) {
		t.Errorf("c and d should equal")
	}
	if !reflect.DeepEqual(a, d) {
		t.Errorf("a and d should equal")
	}

	m := FromCode("\U0001f44d")
	n := FromAlias(":thumbsup:")
	o := FromAlias("+1")

	if !reflect.DeepEqual(m, n) {
		t.Errorf("m and n should equal")
	}
	if !reflect.DeepEqual(n, o) {
		t.Errorf("n and o should equal")
	}
	if !reflect.DeepEqual(m, o) {
		t.Errorf("m and o should equal")
	}
}

func TestReplacers(t *testing.T) {
	tests := []struct {
		f      func(string) string
		v, exp string
	}{
		{ReplaceCodes, ":thumbsup: +1 for \U0001f37a! 🍺 \U0001f44d", ":thumbsup: +1 for :beer:! :beer: :+1:"},
		{ReplaceAliases, ":thumbsup: +1 :+1: :beer:", "\U0001f44d +1 \U0001f44d \U0001f37a"},
	}

	for i, x := range tests {
		s := x.f(x.v)
		if s != x.exp {
			t.Errorf("test %d `%s` expected `%s`, got: `%s`", i, x.v, x.exp, s)
		}
	}
}

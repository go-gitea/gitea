// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDict(t *testing.T) {
	type M map[string]any
	cases := []struct {
		args []any
		want map[string]any
	}{
		{[]any{"a", 1, "b", 2}, M{"a": 1, "b": 2}},
		{[]any{".", M{"base": 1}, "b", 2}, M{"base": 1, "b": 2}},
		{[]any{"a", 1, ".", M{"extra": 2}}, M{"a": 1, "extra": 2}},
		{[]any{"a", 1, ".", map[string]int{"int": 2}}, M{"a": 1, "int": 2}},
		{[]any{".", nil, "b", 2}, M{"b": 2}},
	}

	for _, c := range cases {
		got, err := dict(c.args...)
		if assert.NoError(t, err) {
			assert.EqualValues(t, c.want, got)
		}
	}

	bads := []struct {
		args []any
	}{
		{[]any{"a", 1, "b"}},
		{[]any{1}},
		{[]any{struct{}{}}},
	}
	for _, c := range bads {
		_, err := dict(c.args...)
		assert.Error(t, err)
	}
}

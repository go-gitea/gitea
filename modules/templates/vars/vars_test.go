// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandVars(t *testing.T) {
	kases := []struct {
		tmpl  string
		data  map[string]string
		out   string
		error bool
	}{
		{
			tmpl: "{a}",
			data: map[string]string{
				"a": "1",
			},
			out: "1",
		},
		{
			tmpl: "expand {a}, {b} and {c}, with non-var { } {#}",
			data: map[string]string{
				"a": "1",
				"b": "2",
				"c": "3",
			},
			out: "expand 1, 2 and 3, with non-var { } {#}",
		},
		{
			tmpl: "中文内容 {一}, {二} 和 {三} 中文结尾",
			data: map[string]string{
				"一": "11",
				"二": "22",
				"三": "33",
			},
			out: "中文内容 11, 22 和 33 中文结尾",
		},
		{
			tmpl: "expand {{a}, {b} and {c}",
			data: map[string]string{
				"a": "foo",
				"b": "bar",
			},
			out:   "expand {{a}, bar and {c}",
			error: true,
		},
		{
			tmpl:  "expand } {} and {",
			out:   "expand } {} and {",
			error: true,
		},
	}

	for _, kase := range kases {
		t.Run(kase.tmpl, func(t *testing.T) {
			res, err := Expand(kase.tmpl, kase.data)
			assert.EqualValues(t, kase.out, res)
			if kase.error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

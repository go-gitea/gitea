// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package vars

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandVars(t *testing.T) {
	kases := []struct {
		template string
		maps     map[string]string
		expected string
		fail     bool
	}{
		{
			template: "{a}",
			maps: map[string]string{
				"a": "1",
			},
			expected: "1",
		},
		{
			template: "expand {a}, {b} and {c}, with escaped {#{}",
			maps: map[string]string{
				"a": "1",
				"b": "2",
				"c": "3",
			},
			expected: "expand 1, 2 and 3, with escaped {",
		},
		{
			template: "中文内容 {一}, {二} 和 {三} 中文结尾",
			maps: map[string]string{
				"一": "11",
				"二": "22",
				"三": "33",
			},
			expected: "中文内容 11, 22 和 33 中文结尾",
		},
		{
			template: "expand {{a}, {b} and {c}",
			maps: map[string]string{
				"a": "foo",
				"b": "bar",
			},
			expected: "expand {{a}, bar and {c}",
			fail:     true,
		},
		{
			template: "expand } {} and {",
			expected: "expand } {} and {",
			fail:     true,
		},
	}

	for _, kase := range kases {
		t.Run(kase.template, func(t *testing.T) {
			res, err := Expand(kase.template, kase.maps)
			if kase.fail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.EqualValues(t, kase.expected, res)
		})
	}
}

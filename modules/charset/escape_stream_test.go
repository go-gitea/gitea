// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/modules/translation"

	"github.com/stretchr/testify/assert"
)

func Test_escapeStreamer_isAllowed(t *testing.T) {
	tests := []struct {
		allowed []rune
		r       rune
		want    bool
	}{
		{
			allowed: nil,
			r:       'a',
			want:    false,
		},
		{
			allowed: []rune{'a', 'b', 'c'},
			r:       'x',
			want:    false,
		},
		{
			allowed: []rune{'a', 'b', 'c'},
			r:       'a',
			want:    true,
		},
		{
			allowed: []rune{'c', 'b', 'a'},
			r:       'a',
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v %v", tt.r, tt.allowed), func(t *testing.T) {
			e := NewEscapeStreamer(translation.NewLocale("en"), nil, tt.allowed...).(*escapeStreamer)
			assert.Equalf(t, tt.want, e.isAllowed(tt.r), "isAllowed(%v)", tt.r)
		})
	}
}

// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseSignatureFromCommitLine(t *testing.T) {
	tests := []struct {
		line string
		want *Signature
	}{
		{
			line: "a b <c@d.com> 12345 +0100",
			want: &Signature{
				Name:  "a b",
				Email: "c@d.com",
				When:  time.Unix(12345, 0).In(time.FixedZone("", 3600)),
			},
		},
		{
			line: "bad line",
			want: &Signature{Name: "bad line"},
		},
		{
			line: "bad < line",
			want: &Signature{Name: "bad < line"},
		},
		{
			line: "bad > line",
			want: &Signature{Name: "bad > line"},
		},
		{
			line: "bad-line <name@example.com>",
			want: &Signature{Name: "bad-line <name@example.com>"},
		},
	}
	for _, test := range tests {
		got := parseSignatureFromCommitLine(test.line)
		assert.EqualValues(t, test.want, got)
	}
}

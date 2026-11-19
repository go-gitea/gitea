// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseSignatureFromCommitLine(t *testing.T) {
	assert.Equal(t, &Signature{
		Name:  "a b",
		Email: "c@d.com",
		When:  time.Unix(12345, 0).In(time.FixedZone("", 3600)),
	}, parseSignatureFromCommitLine("a b <c@d.com> 12345 +0100"))

	for _, line := range []string{
		"bad line",
		"bad < line",
		"bad > line",
		"bad-line <name@example.com>",
	} {
		assert.Equal(t, &Signature{Name: line}, parseSignatureFromCommitLine(line))
	}
}

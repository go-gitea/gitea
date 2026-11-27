// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffContentHistory(t *testing.T) {
	out := diffContentHistory("<\r\n&\r\n>", "<\nXXX\n>")
	assert.Equal(t, `<pre>&lt;
<span class="removed-code">&amp;</span><span class="added-code">XXX</span>
&gt;</pre>`, string(out))
}

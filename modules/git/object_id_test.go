// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidSHAPattern(t *testing.T) {
	assert.True(t, Sha1ObjectFormat.IsValid("fee1"))
	assert.True(t, Sha1ObjectFormat.IsValid("abc000"))
	assert.True(t, Sha1ObjectFormat.IsValid("9023902390239023902390239023902390239023"))
	assert.False(t, Sha1ObjectFormat.IsValid("90239023902390239023902390239023902390239023"))
	assert.False(t, Sha1ObjectFormat.IsValid("abc"))
	assert.False(t, Sha1ObjectFormat.IsValid("123g"))
	assert.False(t, Sha1ObjectFormat.IsValid("some random text"))
	assert.Equal(t, "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391", ComputeBlobHash(Sha1ObjectFormat, nil).String())
	assert.Equal(t, "2e65efe2a145dda7ee51d1741299f848e5bf752e", ComputeBlobHash(Sha1ObjectFormat, []byte("a")).String())
	assert.Equal(t, "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813", ComputeBlobHash(Sha256ObjectFormat, nil).String())
	assert.Equal(t, "eb337bcee2061c5313c9a1392116b6c76039e9e30d71467ae359b36277e17dc7", ComputeBlobHash(Sha256ObjectFormat, []byte("a")).String())
	assert.True(t, IsEmptyCommitID(""))
	assert.True(t, IsEmptyCommitID("0000000000000000000000000000000000000000"))
}

func TestInvalidObjectFormat(t *testing.T) {
	of := ObjectFormatFromName("no-such")
	assert.NotNil(t, of)
	assert.False(t, IsValidObjectFormat("no-such"))
}

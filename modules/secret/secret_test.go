// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	result, err := New()
	assert.NoError(t, err)
	assert.True(t, len(result) > 32)

	result2, err := New()
	assert.NoError(t, err)
	// check if secrets
	assert.NotEqual(t, result, result2)
}

// +build pam

// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pam

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPamAuth(t *testing.T) {
	result, err := Auth("gitea", "user1", "false-pwd")
	assert.Error(t, err)
	assert.EqualError(t, err, "Authentication failure")
	assert.Len(t, result, 0)
}

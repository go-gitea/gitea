// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveUsernameParameterSuffix(t *testing.T) {
	assert.Equal(t, "foobar", RemoveUsernameParameterSuffix("foobar (Foo Bar)"))
	assert.Equal(t, "foobar", RemoveUsernameParameterSuffix("foobar"))
	assert.Equal(t, "", RemoveUsernameParameterSuffix(""))
}

func TestIsValidSlackChannel(t *testing.T) {
	tt := []struct {
		channelName string
		expected    bool
	}{
		{"gitea", true},
		{"  ", false},
		{"#", false},
		{"gitea   ", true},
		{"  gitea", true},
	}

	for _, v := range tt {
		assert.Equal(t, v.expected, IsValidSlackChannel(v.channelName))
	}
}

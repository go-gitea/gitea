// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ldap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserAttributeFilter(t *testing.T) {
	const userFilter = "(&(objectClass=posixAccount)(uid=user1))"

	assert.Equal(t, "(objectClass=*)", userAttributeFilter(userFilter, true))
	assert.Equal(t, userFilter, userAttributeFilter(userFilter, false))
}

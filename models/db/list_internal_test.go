// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendPrimaryKeyOrder(t *testing.T) {
	const primaryKey = "`repository`.`id`"
	for _, test := range []struct {
		order, expected string
	}{
		{"", "`repository`.`id` ASC"},
		{"name ASC", "name ASC, `repository`.`id` ASC"},
		{"name DESC", "name DESC, `repository`.`id` DESC"},
		{"name", "name, `repository`.`id` ASC"},
		{"COALESCE(label.exclusive_order, 2147483647) ASC", "COALESCE(label.exclusive_order, 2147483647) ASC, `repository`.`id` ASC"},
		{"name ASC, `repository`.`id` DESC", "name ASC, `repository`.`id` DESC"},
	} {
		assert.Equal(t, test.expected, appendPrimaryKeyOrder(test.order, primaryKey), test.order)
	}
}

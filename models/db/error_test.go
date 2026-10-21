// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type conflictError struct{ sqlState string }

func (err conflictError) Error() string    { return "conflicting update" }
func (err conflictError) SQLState() string { return err.sqlState }

func TestIsConflictError(t *testing.T) {
	assert.True(t, IsConflictError(conflictError{sqlState: "40001"}))
	assert.True(t, IsConflictError(fmt.Errorf("claim job: %w", conflictError{sqlState: "40001"})))
	assert.False(t, IsConflictError(conflictError{sqlState: "23505"}))
	assert.False(t, IsConflictError(errors.New("deadlock")))
	assert.False(t, IsConflictError(nil))
}

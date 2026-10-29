// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsTXTRecordAbsent(t *testing.T) {
	require.True(t, isTXTRecordAbsent(&net.DNSError{IsNotFound: true}))
	require.True(t, isTXTRecordAbsent(fmt.Errorf("wrapped: %w", &net.DNSError{IsNotFound: true})))
	require.False(t, isTXTRecordAbsent(&net.DNSError{IsTimeout: true, IsTemporary: true}))
	require.False(t, isTXTRecordAbsent(errors.New("resolver unavailable")))
}

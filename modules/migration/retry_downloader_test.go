// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryDownloaderExhaustsAttempts(t *testing.T) {
	failure := errors.New("permission denied")
	attempts := 0
	assert.ErrorIs(t, NewRetryDownloader(nil, 3, 0).retry(t.Context(), func(_ctx context.Context) error {
		attempts++
		return failure
	}), failure)
	assert.Equal(t, 3, attempts)
}

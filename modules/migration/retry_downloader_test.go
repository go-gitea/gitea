// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type failingCommentsDownloader struct {
	NullDownloader
	attempts int
}

func (d *failingCommentsDownloader) GetAllComments(_ctx context.Context, _page, _perPage int) ([]*Comment, bool, error) {
	d.attempts++
	return nil, false, errors.New("permission denied")
}

func TestRetryDownloaderExhaustsAttempts(t *testing.T) {
	source := &failingCommentsDownloader{}
	_, _, err := NewRetryDownloader(source, 3, 0).GetAllComments(t.Context(), 1, 50)
	assert.EqualError(t, err, "permission denied")
	assert.Equal(t, 3, source.attempts)
}

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

type flakyCommentsDownloader struct {
	NullDownloader
	calls int
}

func (d *flakyCommentsDownloader) GetAllComments(_ context.Context, page, _ int) ([]*Comment, bool, error) {
	d.calls++
	if d.calls == 1 {
		return nil, false, errors.New("temporary failure")
	}
	return []*Comment{{Index: int64(page)}}, true, nil
}

func TestRetryDownloaderGetAllComments(t *testing.T) {
	source := &flakyCommentsDownloader{}
	comments, isEnd, err := NewRetryDownloader(source, 2, 0).GetAllComments(t.Context(), 3, 50)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.Equal(t, []*Comment{{Index: 3}}, comments)
	assert.Equal(t, 2, source.calls)
}

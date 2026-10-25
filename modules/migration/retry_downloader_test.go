// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type commentRetryDownloader struct {
	NullDownloader
	calls int
}

func (d *commentRetryDownloader) GetAllComments(_ context.Context, page, perPage int) ([]*Comment, bool, error) {
	d.calls++
	if d.calls == 1 {
		return nil, false, errors.New("temporary failure")
	}
	return []*Comment{{Index: int64(page), IssueIndex: int64(perPage)}}, true, nil
}

func TestRetryDownloaderGetAllComments(t *testing.T) {
	source := &commentRetryDownloader{}
	downloader := NewRetryDownloader(source, 2, 0)
	comments, end, err := downloader.GetAllComments(t.Context(), 3, 50)
	require.NoError(t, err)
	assert.True(t, end)
	require.Len(t, comments, 1)
	assert.Equal(t, int64(3), comments[0].Index)
	assert.Equal(t, int64(50), comments[0].IssueIndex)
	assert.Equal(t, 2, source.calls)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err = downloader.GetAllComments(ctx, 3, 50)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 2, source.calls)
}

func TestRetryDownloaderExhaustsAttempts(t *testing.T) {
	failure := errors.New("permission denied")
	attempts := 0
	assert.ErrorIs(t, NewRetryDownloader(nil, 3, 0).retry(t.Context(), func(_ctx context.Context) error {
		attempts++
		return failure
	}), failure)
	assert.Equal(t, 3, attempts)
}

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package imagecaptcha

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/modules/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageCaptcha(t *testing.T) {
	require.NoError(t, cache.Init())
	createWithAnswer := func() (string, string) {
		id, err := CreateNew()
		require.NoError(t, err)
		code, ok := cache.GetCache().Get(cacheKeyPrefix + id)
		require.True(t, ok)
		return id, code
	}
	renderImage := func(id string, refresh bool) *httptest.ResponseRecorder {
		resp := httptest.NewRecorder()
		reqLink := "/captcha?id=" + id
		if refresh {
			reqLink += "&reload=any"
		}
		ServeImage(resp, httptest.NewRequest(http.MethodGet, reqLink, nil))
		return resp
	}
	id, answer := createWithAnswer()
	assert.Len(t, answer, codeLength)
	assert.True(t, Verify(id, answer))
	assert.False(t, Verify(id, answer))

	id, answer = createWithAnswer()
	assert.False(t, Verify(id, "wrong"))
	assert.False(t, Verify(id, answer))
	assert.False(t, Verify("", ""))
	assert.False(t, Verify("unknown", answer))

	resp := renderImage("unknown", true)
	assert.Equal(t, http.StatusNotFound, resp.Code)
	_, exists := cache.GetCache().Get(cacheKeyPrefix + "unknown")
	assert.False(t, exists)

	id, _ = createWithAnswer()
	first := renderImage(id, false)
	decoded, err := png.Decode(bytes.NewReader(first.Body.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, imageWidth, imageHeight), decoded.Bounds())
	second := renderImage(id, false)
	assert.Equal(t, first.Body.Bytes(), second.Body.Bytes())

	require.NoError(t, cache.GetCache().Delete(cacheKeyPrefix+id))
	_ = renderImage(id, true)
	_, exists = cache.GetCache().Get(cacheKeyPrefix + id)
	assert.True(t, exists)
}

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jsonlimit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestDecode(t *testing.T) {
	for name, decode := range map[string]func(io.Reader, int64, any) error{
		"reader": DecodeWithLimit,
		"request": func(r io.Reader, limit int64, v any) error {
			return DecodeFromRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", r), limit, v)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var v map[string]string
			assert.NoError(t, decode(strings.NewReader(`{"a":"b"}`), 9, &v))
			assert.Equal(t, map[string]string{"a": "b"}, v)

			err := decode(strings.NewReader(`{"a":"bc"}`), 9, &v)
			assert.ErrorIs(t, err, util.ErrInvalidArgument)

			err = decode(strings.NewReader(`{"a":`), 9, &v)
			assert.Error(t, err)
			assert.NotErrorIs(t, err, util.ErrInvalidArgument)
		})
	}
}

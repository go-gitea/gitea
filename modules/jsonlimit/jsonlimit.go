// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jsonlimit

import (
	"errors"
	"io"
	"net/http"

	"gitea.dev/modules/json"
	"gitea.dev/modules/util"
)

// DecodeWithLimit decodes JSON from reader into v with a size limit.
// Oversized input returns an error wrapping util.ErrInvalidArgument.
func DecodeWithLimit(reader io.Reader, limit int64, v any) error {
	lr := &io.LimitedReader{R: reader, N: limit + 1}
	err := json.NewDecoder(lr).Decode(v)
	if lr.N <= 0 {
		return util.NewInvalidArgumentErrorf("json: input exceeds the size limit: %d bytes", limit)
	}
	return err
}

func DecodeFromRequest(w http.ResponseWriter, r *http.Request, limit int64, v any) error {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit)).Decode(v)
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return util.NewInvalidArgumentErrorf("json: request exceeds the size limit: %d bytes", limit)
	}
	return err
}

// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !goexperiment.jsonv2

package json

import (
	"io"
)

func getDefaultJSONHandler() Interface {
	return jsonGoccy{}
}

func MarshalKeepOptionalEmpty(v any) ([]byte, error) {
	return DefaultJSONHandler.Marshal(v)
}

func NewDecoderCaseInsensitive(reader io.Reader) Decoder {
	return DefaultJSONHandler.NewDecoder(reader)
}

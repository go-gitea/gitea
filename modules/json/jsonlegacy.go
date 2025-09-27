// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !goexperiment.jsonv2

package json

import (
	"io"

	jsoniter "github.com/json-iterator/go"
)

func getDefaultJSONHandler() Interface {
	return JSONiter{jsoniter.ConfigCompatibleWithStandardLibrary}
}

func MarshalKeepOptionalEmpty(v any) ([]byte, error) {
	return DefaultJSONHandler.Marshal(v)
}

func NewDecoderCaseInsensitive(reader io.Reader) Decoder {
	return DefaultJSONHandler.NewDecoder(reader)
}

// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !goexperiment.jsonv2

package json

import jsoniter "github.com/json-iterator/go"

func getDefaultJSONHandler() Interface {
	return JSONiter{jsoniter.ConfigCompatibleWithStandardLibrary}
}

func MarshalKeepOptionalEmpty(v any) ([]byte, error) {
	return DefaultJSONHandler.Marshal(v)
}

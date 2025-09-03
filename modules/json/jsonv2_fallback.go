// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !goexperiment.jsonv2
package json

import "io"

// isJSONv2Available returns false when JSON v2 is not available (not compiled with GOEXPERIMENT=jsonv2)
func isJSONv2Available() bool {
	return false
}

// marshalV2 fallback - should not be called when JSON v2 is not available
func marshalV2(v any) ([]byte, error) {
	panic("JSON v2 not available - build with GOEXPERIMENT=jsonv2")
}

// unmarshalV2 fallback - should not be called when JSON v2 is not available
func unmarshalV2(data []byte, v any) error {
	panic("JSON v2 not available - build with GOEXPERIMENT=jsonv2")
}

// newEncoderV2 fallback - should not be called when JSON v2 is not available
func newEncoderV2(writer io.Writer) Encoder {
	panic("JSON v2 not available - build with GOEXPERIMENT=jsonv2")
}

// newDecoderV2 fallback - should not be called when JSON v2 is not available
func newDecoderV2(reader io.Reader) Decoder {
	panic("JSON v2 not available - build with GOEXPERIMENT=jsonv2")
}

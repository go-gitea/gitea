// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"bytes"
	"encoding/gob"
)

// PackData uses gob to encode the given data in sequence
func PackData(data ...any) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, datum := range data {
		if err := enc.Encode(datum); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// UnpackData uses gob to decode the given data in sequence
func UnpackData(buf []byte, data ...any) error {
	r := bytes.NewReader(buf)
	enc := gob.NewDecoder(r)
	for _, datum := range data {
		if err := enc.Decode(datum); err != nil {
			return err
		}
	}
	return nil
}

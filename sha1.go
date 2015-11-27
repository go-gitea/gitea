// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"encoding/hex"
	"fmt"
	"strings"
)

type sha1 [20]byte

// String returns string (hex) representation of the Oid.
func (s sha1) String() string {
	result := make([]byte, 0, 40)
	hexvalues := []byte("0123456789abcdef")
	for i := 0; i < 20; i++ {
		result = append(result, hexvalues[s[i]>>4])
		result = append(result, hexvalues[s[i]&0xf])
	}
	return string(result)
}

// NewID creates a new sha1 from a [20]byte array.
func NewID(b []byte) (sha1, error) {
	var id sha1
	if len(b) != 20 {
		return id, fmt.Errorf("Length must be 20: %v", b)
	}

	for i := 0; i < 20; i++ {
		id[i] = b[i]
	}
	return id, nil
}

// NewIDFromString creates a new sha1 from a ID string of length 40.
func NewIDFromString(s string) (sha1, error) {
	var id sha1
	s = strings.TrimSpace(s)
	if len(s) != 40 {
		return id, fmt.Errorf("Length must be 40: %s", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return id, err
	}
	return NewID(b)
}

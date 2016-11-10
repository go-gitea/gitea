// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const EMPTY_SHA = "0000000000000000000000000000000000000000"

type sha1 [20]byte

// Equal returns true if s has the same sha1 as caller.
// Support 40-length-string, []byte, sha1.
func (id sha1) Equal(s2 interface{}) bool {
	switch v := s2.(type) {
	case string:
		if len(v) != 40 {
			return false
		}
		return v == id.String()
	case []byte:
		if len(v) != 20 {
			return false
		}
		for i, v := range v {
			if id[i] != v {
				return false
			}
		}
	case sha1:
		for i, v := range v {
			if id[i] != v {
				return false
			}
		}
	default:
		return false
	}
	return true
}

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

// MustID always creates a new sha1 from a [20]byte array with no validation of input.
func MustID(b []byte) sha1 {
	var id sha1
	for i := 0; i < 20; i++ {
		id[i] = b[i]
	}
	return id
}

// NewID creates a new sha1 from a [20]byte array.
func NewID(b []byte) (sha1, error) {
	if len(b) != 20 {
		return sha1{}, fmt.Errorf("Length must be 20: %v", b)
	}
	return MustID(b), nil
}

// MustIDFromString always creates a new sha from a ID with no validation of input.
func MustIDFromString(s string) sha1 {
	b, _ := hex.DecodeString(s)
	return MustID(b)
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

// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kv

import "bytes"

// Key represents high-level Key type.
type Key []byte

// Next returns the next key in byte-order.
func (k Key) Next() Key {
	// add 0x0 to the end of key
	buf := make([]byte, len([]byte(k))+1)
	copy(buf, []byte(k))
	return buf
}

// Cmp returns the comparison result of two key.
// The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
func (k Key) Cmp(another Key) int {
	return bytes.Compare(k, another)
}

// HasPrefix tests whether the Key begins with prefix.
func (k Key) HasPrefix(prefix Key) bool {
	return bytes.HasPrefix(k, prefix)
}

// Clone returns a copy of the Key.
func (k Key) Clone() Key {
	return append([]byte(nil), k...)
}

// EncodedKey represents encoded key in low-level storage engine.
type EncodedKey []byte

// Cmp returns the comparison result of two key.
// The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
func (k EncodedKey) Cmp(another EncodedKey) int {
	return bytes.Compare(k, another)
}

// Next returns the next key in byte-order.
func (k EncodedKey) Next() EncodedKey {
	return EncodedKey(bytes.Join([][]byte{k, Key{0}}, nil))
}

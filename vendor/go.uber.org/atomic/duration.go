// @generated Code generated by gen-atomicwrapper.

// Copyright (c) 2020-2021 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package atomic

import (
	"encoding/json"
	"time"
)

// Duration is an atomic type-safe wrapper for time.Duration values.
type Duration struct {
	_ nocmp // disallow non-atomic comparison

	v Int64
}

var _zeroDuration time.Duration

// NewDuration creates a new Duration.
func NewDuration(val time.Duration) *Duration {
	x := &Duration{}
	if val != _zeroDuration {
		x.Store(val)
	}
	return x
}

// Load atomically loads the wrapped time.Duration.
func (x *Duration) Load() time.Duration {
	return time.Duration(x.v.Load())
}

// Store atomically stores the passed time.Duration.
func (x *Duration) Store(val time.Duration) {
	x.v.Store(int64(val))
}

// CAS is an atomic compare-and-swap for time.Duration values.
func (x *Duration) CAS(old, new time.Duration) (swapped bool) {
	return x.v.CAS(int64(old), int64(new))
}

// Swap atomically stores the given time.Duration and returns the old
// value.
func (x *Duration) Swap(val time.Duration) (old time.Duration) {
	return time.Duration(x.v.Swap(int64(val)))
}

// MarshalJSON encodes the wrapped time.Duration into JSON.
func (x *Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}

// UnmarshalJSON decodes a time.Duration from JSON.
func (x *Duration) UnmarshalJSON(b []byte) error {
	var v time.Duration
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	x.Store(v)
	return nil
}

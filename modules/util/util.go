// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"
)

// OptionalBool a boolean that can be "null"
type OptionalBool byte

const (
	// OptionalBoolNone a "null" boolean value
	OptionalBoolNone = iota
	// OptionalBoolTrue a "true" boolean value
	OptionalBoolTrue
	// OptionalBoolFalse a "false" boolean value
	OptionalBoolFalse
)

// IsTrue return true if equal to OptionalBoolTrue
func (o OptionalBool) IsTrue() bool {
	return o == OptionalBoolTrue
}

// IsFalse return true if equal to OptionalBoolFalse
func (o OptionalBool) IsFalse() bool {
	return o == OptionalBoolFalse
}

// IsNone return true if equal to OptionalBoolNone
func (o OptionalBool) IsNone() bool {
	return o == OptionalBoolNone
}

// OptionalBoolOf get the corresponding OptionalBool of a bool
func OptionalBoolOf(b bool) OptionalBool {
	if b {
		return OptionalBoolTrue
	}
	return OptionalBoolFalse
}

// Max max of two ints
func Max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// Min min of two ints
func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// IsEmptyString checks if the provided string is empty
func IsEmptyString(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

type normalizeEOLReader struct {
	rd           io.Reader
	isLastReturn bool
}

func (r *normalizeEOLReader) Read(bs []byte) (int, error) {
	var p = make([]byte, len(bs))
	n, err := r.rd.Read(p)
	if err != nil {
		return n, err
	}

	var j = 0
	for i, c := range p[:n] {
		if i == 0 {
			if c == '\n' && r.isLastReturn {
				r.isLastReturn = false
				continue
			}
			r.isLastReturn = false
		}
		if c == '\r' {
			if i < n-1 {
				if p[i+1] != '\n' {
					bs[j] = '\n'
				} else {
					continue
				}
			} else {
				r.isLastReturn = true
				bs[j] = '\n'
			}
		} else {
			bs[j] = c
		}
		j++
	}

	return j, nil
}

// NormalizeEOLReader will convert Windows (CRLF) and Mac (CR) EOLs to UNIX (LF) from a reader
func NormalizeEOLReader(rd io.Reader) io.Reader {
	return &normalizeEOLReader{
		rd:           rd,
		isLastReturn: false,
	}
}

// NormalizeEOL will convert Windows (CRLF) and Mac (CR) EOLs to UNIX (LF)
func NormalizeEOL(input []byte) []byte {
	bs, _ := ioutil.ReadAll(NormalizeEOLReader(bytes.NewReader(input)))
	return bs
}

// MergeInto merges pairs of values into a "dict"
func MergeInto(dict map[string]interface{}, values ...interface{}) (map[string]interface{}, error) {
	for i := 0; i < len(values); i++ {
		switch key := values[i].(type) {
		case string:
			i++
			if i == len(values) {
				return nil, errors.New("specify the key for non array values")
			}
			dict[key] = values[i]
		case map[string]interface{}:
			m := values[i].(map[string]interface{})
			for i, v := range m {
				dict[i] = v
			}
		default:
			return nil, errors.New("dict values must be maps")
		}
	}

	return dict, nil
}

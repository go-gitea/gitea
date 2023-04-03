// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"io"
)

// ReadAtMost reads at most len(buf) bytes from r into buf.
// It returns the number of bytes copied. n is only less than len(buf) if r provides fewer bytes.
// If EOF occurs while reading, err will be nil.
func ReadAtMost(r io.Reader, buf []byte) (n int, err error) {
	n, err = io.ReadFull(r, buf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	return n, err
}

// ErrNotEmpty is an error reported when there is a non-empty reader
var ErrNotEmpty = errors.New("not-empty")

// IsEmptyReader reads a reader and ensures it is empty
func IsEmptyReader(r io.Reader) (err error) {
	var buf [1]byte

	for {
		n, err := r.Read(buf[:])
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if n > 0 {
			return ErrNotEmpty
		}
	}
}

// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"bytes"
	"errors"
	"io"
)

// ReadAtMost reads at most len(buf) bytes from r into buf.
// It returns the number of bytes copied. n is only less than len(buf) if r provides fewer bytes.
// If EOF or ErrUnexpectedEOF occurs while reading, err will be nil.
func ReadAtMost(r io.Reader, buf []byte) (n int, err error) {
	n, err = io.ReadFull(r, buf)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	return n, err
}

// ReadWithLimit reads at most "limit" bytes from r into buf.
// If EOF or ErrUnexpectedEOF occurs while reading, err will be nil.
func ReadWithLimit(r io.Reader, n int) (buf []byte, err error) {
	return readWithLimit(r, 1024, n)
}

func readWithLimit(r io.Reader, batch, limit int) ([]byte, error) {
	if limit <= batch {
		buf := make([]byte, limit)
		n, err := ReadAtMost(r, buf)
		if err != nil {
			return nil, err
		}
		return buf[:n], nil
	}
	res := bytes.NewBuffer(make([]byte, 0, batch))
	bufFix := make([]byte, batch)
	eof := false
	for res.Len() < limit && !eof {
		bufTmp := bufFix
		if res.Len()+batch > limit {
			bufTmp = bufFix[:limit-res.Len()]
		}
		n, err := io.ReadFull(r, bufTmp)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			eof = true
		} else if err != nil {
			return nil, err
		}
		if _, err = res.Write(bufTmp[:n]); err != nil {
			return nil, err
		}
	}
	return res.Bytes(), nil
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

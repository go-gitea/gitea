// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding"
	"errors"
	"hash"
	"io"
)

const (
	marshaledSizeMD5    = 92
	marshaledSizeSHA1   = 96
	marshaledSizeSHA256 = 108
	marshaledSizeSHA512 = 204

	marshaledSize = marshaledSizeMD5 + marshaledSizeSHA1 + marshaledSizeSHA256 + marshaledSizeSHA512
)

// HashSummer provide a Sums method
type HashSummer interface {
	Sums() (hashMD5, hashSHA1, hashSHA256, hashSHA512 []byte)
}

// MultiHasher calculates multiple checksums
type MultiHasher struct {
	md5    hash.Hash
	sha1   hash.Hash
	sha256 hash.Hash
	sha512 hash.Hash

	combinedWriter io.Writer
}

// NewMultiHasher creates a multi hasher
func NewMultiHasher() *MultiHasher {
	md5 := md5.New()
	sha1 := sha1.New()
	sha256 := sha256.New()
	sha512 := sha512.New()

	combinedWriter := io.MultiWriter(md5, sha1, sha256, sha512)

	return &MultiHasher{
		md5,
		sha1,
		sha256,
		sha512,
		combinedWriter,
	}
}

// MarshalBinary implements encoding.BinaryMarshaler
func (h *MultiHasher) MarshalBinary() ([]byte, error) {
	md5Bytes, err := h.md5.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return nil, err
	}
	sha1Bytes, err := h.sha1.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return nil, err
	}
	sha256Bytes, err := h.sha256.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return nil, err
	}
	sha512Bytes, err := h.sha512.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return nil, err
	}

	b := make([]byte, 0, marshaledSize)
	b = append(b, md5Bytes...)
	b = append(b, sha1Bytes...)
	b = append(b, sha256Bytes...)
	b = append(b, sha512Bytes...)
	return b, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (h *MultiHasher) UnmarshalBinary(b []byte) error {
	if len(b) != marshaledSize {
		return errors.New("invalid hash state size")
	}

	if err := h.md5.(encoding.BinaryUnmarshaler).UnmarshalBinary(b[:marshaledSizeMD5]); err != nil {
		return err
	}

	b = b[marshaledSizeMD5:]
	if err := h.sha1.(encoding.BinaryUnmarshaler).UnmarshalBinary(b[:marshaledSizeSHA1]); err != nil {
		return err
	}

	b = b[marshaledSizeSHA1:]
	if err := h.sha256.(encoding.BinaryUnmarshaler).UnmarshalBinary(b[:marshaledSizeSHA256]); err != nil {
		return err
	}

	b = b[marshaledSizeSHA256:]
	return h.sha512.(encoding.BinaryUnmarshaler).UnmarshalBinary(b[:marshaledSizeSHA512])
}

// Write implements io.Writer
func (h *MultiHasher) Write(p []byte) (int, error) {
	return h.combinedWriter.Write(p)
}

// Sums gets the MD5, SHA1, SHA256 and SHA512 checksums of the data
func (h *MultiHasher) Sums() (hashMD5, hashSHA1, hashSHA256, hashSHA512 []byte) {
	hashMD5 = h.md5.Sum(nil)
	hashSHA1 = h.sha1.Sum(nil)
	hashSHA256 = h.sha256.Sum(nil)
	hashSHA512 = h.sha512.Sum(nil)
	return hashMD5, hashSHA1, hashSHA256, hashSHA512
}

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package s2k implements the various OpenPGP string-to-key transforms as
// specified in RFC 4800 section 3.7.1.
package s2k // import "github.com/ProtonMail/go-crypto/openpgp/s2k"

import (
	"crypto"
	"hash"
	"io"
	"strconv"

	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/algorithm"
)

// Config collects configuration parameters for s2k key-stretching
// transformations. A nil *Config is valid and results in all default
// values. Currently, Config is used only by the Serialize function in
// this package.
type Config struct {
	// S2KMode is the mode of s2k function.
	// It can be 0 (simple), 1(salted), 3(iterated)
	// 2(reserved) 100-110(private/experimental).
	S2KMode uint8
	// Hash is the default hash function to be used. If
	// nil, SHA256 is used.
	Hash crypto.Hash
	// S2KCount is only used for symmetric encryption. It
	// determines the strength of the passphrase stretching when
	// the said passphrase is hashed to produce a key. S2KCount
	// should be between 65536 and 65011712, inclusive. If Config
	// is nil or S2KCount is 0, the value 16777216 used. Not all
	// values in the above range can be represented. S2KCount will
	// be rounded up to the next representable value if it cannot
	// be encoded exactly. See RFC 4880 Section 3.7.1.3.
	S2KCount int
}

// Params contains all the parameters of the s2k packet
type Params struct {
	// mode is the mode of s2k function.
	// It can be 0 (simple), 1(salted), 3(iterated)
	// 2(reserved) 100-110(private/experimental).
	mode uint8
	// hashId is the ID of the hash function used in any of the modes
	hashId byte
	// salt is a byte array to use as a salt in hashing process
	salt []byte
	// countByte is used to determine how many rounds of hashing are to
	// be performed in s2k mode 3. See RFC 4880 Section 3.7.1.3.
	countByte byte
}

func (c *Config) hash() crypto.Hash {
	if c == nil || uint(c.Hash) == 0 {
		return crypto.SHA256
	}

	return c.Hash
}

// EncodedCount get encoded count
func (c *Config) EncodedCount() uint8 {
	if c == nil || c.S2KCount == 0 {
		return 224 // The common case. Corresponding to 16777216
	}

	i := c.S2KCount

	switch {
	case i < 65536:
		i = 65536
	case i > 65011712:
		i = 65011712
	}

	return encodeCount(i)
}

// encodeCount converts an iterative "count" in the range 1024 to
// 65011712, inclusive, to an encoded count. The return value is the
// octet that is actually stored in the GPG file. encodeCount panics
// if i is not in the above range (encodedCount above takes care to
// pass i in the correct range). See RFC 4880 Section 3.7.7.1.
func encodeCount(i int) uint8 {
	if i < 65536 || i > 65011712 {
		panic("count arg i outside the required range")
	}

	for encoded := 96; encoded < 256; encoded++ {
		count := decodeCount(uint8(encoded))
		if count >= i {
			return uint8(encoded)
		}
	}

	return 255
}

// decodeCount returns the s2k mode 3 iterative "count" corresponding to
// the encoded octet c.
func decodeCount(c uint8) int {
	return (16 + int(c&15)) << (uint32(c>>4) + 6)
}

// Simple writes to out the result of computing the Simple S2K function (RFC
// 4880, section 3.7.1.1) using the given hash and input passphrase.
func Simple(out []byte, h hash.Hash, in []byte) {
	Salted(out, h, in, nil)
}

var zero [1]byte

// Salted writes to out the result of computing the Salted S2K function (RFC
// 4880, section 3.7.1.2) using the given hash, input passphrase and salt.
func Salted(out []byte, h hash.Hash, in []byte, salt []byte) {
	done := 0
	var digest []byte

	for i := 0; done < len(out); i++ {
		h.Reset()
		for j := 0; j < i; j++ {
			h.Write(zero[:])
		}
		h.Write(salt)
		h.Write(in)
		digest = h.Sum(digest[:0])
		n := copy(out[done:], digest)
		done += n
	}
}

// Iterated writes to out the result of computing the Iterated and Salted S2K
// function (RFC 4880, section 3.7.1.3) using the given hash, input passphrase,
// salt and iteration count.
func Iterated(out []byte, h hash.Hash, in []byte, salt []byte, count int) {
	combined := make([]byte, len(in)+len(salt))
	copy(combined, salt)
	copy(combined[len(salt):], in)

	if count < len(combined) {
		count = len(combined)
	}

	done := 0
	var digest []byte
	for i := 0; done < len(out); i++ {
		h.Reset()
		for j := 0; j < i; j++ {
			h.Write(zero[:])
		}
		written := 0
		for written < count {
			if written+len(combined) > count {
				todo := count - written
				h.Write(combined[:todo])
				written = count
			} else {
				h.Write(combined)
				written += len(combined)
			}
		}
		digest = h.Sum(digest[:0])
		n := copy(out[done:], digest)
		done += n
	}
}

// Generate generates valid parameters from given configuration.
// It will enforce salted + hashed s2k method
func Generate(rand io.Reader, c *Config) (*Params, error) {
	hashId, ok := HashToHashId(c.Hash)
	if !ok {
		return nil, errors.UnsupportedError("no such hash")
	}

	params := &Params{
		mode:      3, // Enforce iterared + salted method
		hashId:    hashId,
		salt:      make([]byte, 8),
		countByte: c.EncodedCount(),
	}

	if _, err := io.ReadFull(rand, params.salt); err != nil {
		return nil, err
	}

	return params, nil
}

// Parse reads a binary specification for a string-to-key transformation from r
// and returns a function which performs that transform. If the S2K is a special
// GNU extension that indicates that the private key is missing, then the error
// returned is errors.ErrDummyPrivateKey.
func Parse(r io.Reader) (f func(out, in []byte), err error) {
	params, err := ParseIntoParams(r)
	if err != nil {
		return nil, err
	}

	return params.Function()
}

// ParseIntoParams reads a binary specification for a string-to-key
// transformation from r and returns a struct describing the s2k parameters.
func ParseIntoParams(r io.Reader) (params *Params, err error) {
	var buf [9]byte

	_, err = io.ReadFull(r, buf[:2])
	if err != nil {
		return
	}

	params = &Params{
		mode:   buf[0],
		hashId: buf[1],
	}

	switch params.mode {
	case 0:
		return params, nil
	case 1:
		_, err = io.ReadFull(r, buf[:8])
		if err != nil {
			return nil, err
		}

		params.salt = buf[:8]
		return params, nil
	case 3:
		_, err = io.ReadFull(r, buf[:9])
		if err != nil {
			return nil, err
		}

		params.salt = buf[:8]
		params.countByte = buf[8]
		return params, nil
	case 101:
		// This is a GNU extension. See
		// https://git.gnupg.org/cgi-bin/gitweb.cgi?p=gnupg.git;a=blob;f=doc/DETAILS;h=fe55ae16ab4e26d8356dc574c9e8bc935e71aef1;hb=23191d7851eae2217ecdac6484349849a24fd94a#l1109
		if _, err = io.ReadFull(r, buf[:4]); err != nil {
			return nil, err
		}
		if buf[0] == 'G' && buf[1] == 'N' && buf[2] == 'U' && buf[3] == 1 {
			return params, nil
		}
		return nil, errors.UnsupportedError("GNU S2K extension")
	}

	return nil, errors.UnsupportedError("S2K function")
}

func (params *Params) Dummy() bool {
	return params != nil && params.mode == 101
}

func (params *Params) Function() (f func(out, in []byte), err error) {
	if params.Dummy() {
		return nil, errors.ErrDummyPrivateKey("dummy key found")
	}
	hashObj, ok := HashIdToHash(params.hashId)
	if !ok {
		return nil, errors.UnsupportedError("hash for S2K function: " + strconv.Itoa(int(params.hashId)))
	}
	if !hashObj.Available() {
		return nil, errors.UnsupportedError("hash not available: " + strconv.Itoa(int(hashObj)))
	}

	switch params.mode {
	case 0:
		f := func(out, in []byte) {
			Simple(out, hashObj.New(), in)
		}

		return f, nil
	case 1:
		f := func(out, in []byte) {
			Salted(out, hashObj.New(), in, params.salt)
		}

		return f, nil
	case 3:
		f := func(out, in []byte) {
			Iterated(out, hashObj.New(), in, params.salt, decodeCount(params.countByte))
		}

		return f, nil
	}

	return nil, errors.UnsupportedError("S2K function")
}

func (params *Params) Serialize(w io.Writer) (err error) {
	if _, err = w.Write([]byte{params.mode}); err != nil {
		return
	}
	if _, err = w.Write([]byte{params.hashId}); err != nil {
		return
	}
	if params.Dummy() {
		_, err = w.Write(append([]byte("GNU"), 1))
		return
	}
	if params.mode > 0 {
		if _, err = w.Write(params.salt); err != nil {
			return
		}
		if params.mode == 3 {
			_, err = w.Write([]byte{params.countByte})
		}
	}
	return
}

// Serialize salts and stretches the given passphrase and writes the
// resulting key into key. It also serializes an S2K descriptor to
// w. The key stretching can be configured with c, which may be
// nil. In that case, sensible defaults will be used.
func Serialize(w io.Writer, key []byte, rand io.Reader, passphrase []byte, c *Config) error {
	params, err := Generate(rand, c)
	if err != nil {
		return err
	}
	err = params.Serialize(w)
	if err != nil {
		return err
	}

	f, err := params.Function()
	if err != nil {
		return err
	}
	f(key, passphrase)
	return nil
}

// HashIdToHash returns a crypto.Hash which corresponds to the given OpenPGP
// hash id.
func HashIdToHash(id byte) (h crypto.Hash, ok bool) {
	if hash, ok := algorithm.HashById[id]; ok {
		return hash.HashFunc(), true
	}
	return 0, false
}

// HashIdToString returns the name of the hash function corresponding to the
// given OpenPGP hash id.
func HashIdToString(id byte) (name string, ok bool) {
	if hash, ok := algorithm.HashById[id]; ok {
		return hash.String(), true
	}
	return "", false
}

// HashIdToHash returns an OpenPGP hash id which corresponds the given Hash.
func HashToHashId(h crypto.Hash) (id byte, ok bool) {
	for id, hash := range algorithm.HashById {
		if hash.HashFunc() == h {
			return id, true
		}
	}
	return 0, false
}

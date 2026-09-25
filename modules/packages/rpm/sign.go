// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"crypto"
	"encoding/binary"
	"io"
	"maps"
	"slices"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// SignPackage replaces all OpenPGP signatures with a header-only and a header+payload one, the latter needed by packages without payload digest
func SignPackage(r io.ReadSeeker, key *packet.PrivateKey) (io.Reader, error) {
	lead, sig, h, err := readHeaders(r)
	if err != nil {
		return nil, err
	}
	payloadStart, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	headerSignature, err := signOpenPGP(key, bytes.NewReader(h.raw))
	if err != nil {
		return nil, err
	}
	packageSignature, err := signOpenPGP(key, io.MultiReader(bytes.NewReader(h.raw), r))
	if err != nil {
		return nil, err
	}
	if _, err := r.Seek(payloadStart, io.SeekStart); err != nil {
		return nil, err
	}

	for _, tag := range []uint32{sigTagDSA, sigTagGPG, sigTagReservedSpace} {
		delete(sig.entries, tag)
	}
	sig.entries[sigTagRSA] = headerEntry{typ: typeBin, count: uint32(len(headerSignature)), data: headerSignature}
	sig.entries[sigTagPGP] = headerEntry{typ: typeBin, count: uint32(len(packageSignature)), data: packageSignature}

	return io.MultiReader(bytes.NewReader(lead), bytes.NewReader(sig.marshalSignature()), bytes.NewReader(h.raw), r), nil
}

func signOpenPGP(key *packet.PrivateKey, data io.Reader) ([]byte, error) {
	digest := crypto.SHA256.New()
	if _, err := io.Copy(digest, data); err != nil {
		return nil, err
	}
	signature := &packet.Signature{
		SigType:      packet.SigTypeBinary,
		CreationTime: time.Now(),
		PubKeyAlgo:   key.PubKeyAlgo,
		Hash:         crypto.SHA256,
		IssuerKeyId:  &key.KeyId,
	}
	if err := signature.Sign(digest, key, nil); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err := signature.Serialize(&buf)
	return buf.Bytes(), err
}

// marshalSignature serializes a signature header like rpmsign: region entry first, data in tag order, region trailer last
func (h *header) marshalSignature() []byte {
	tags := slices.DeleteFunc(slices.Sorted(maps.Keys(h.entries)), func(tag uint32) bool { return tag <= maxRegionTag })
	var index, store []byte
	for _, tag := range tags {
		e := h.entries[tag]
		if size := typeSizes[e.typ]; size > 1 {
			store = append(store, make([]byte, (size-len(store)%size)%size)...)
		}
		index = appendUint32s(index, tag, e.typ, uint32(len(store)), e.count)
		store = append(store, e.data...)
	}

	count := uint32(len(tags) + 1)
	region := appendUint32s(nil, sigTagRegion, typeBin, uint32(len(store)), indexEntrySize)
	store = appendUint32s(store, sigTagRegion, typeBin, -count*indexEntrySize, indexEntrySize)
	out := appendUint32s(nil, headerMagic, 0, count, uint32(len(store)))
	out = append(append(append(out, region...), index...), store...)
	return append(out, make([]byte, signaturePadding(len(store)))...)
}

func appendUint32s(b []byte, values ...uint32) []byte {
	for _, v := range values {
		b = binary.BigEndian.AppendUint32(b, v)
	}
	return b
}

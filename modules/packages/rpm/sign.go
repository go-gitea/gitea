// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"crypto"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"io"
	"maps"
	"slices"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

var payloadDigestHashes = map[uint64]func() hash.Hash{1: md5.New, 2: sha1.New, 8: sha256.New, 9: sha512.New384, 10: sha512.New, 11: sha256.New224} // rpm's PGPHASHALGO ids

// SignPackage verifies the payload and replaces all OpenPGP signatures with a header-only and a header+payload one, the latter needed by packages without payload digest
func SignPackage(r io.ReadSeeker, key *packet.PrivateKey) (io.Reader, error) {
	lead, sig, h, err := readHeaders(r)
	if err != nil {
		return nil, err
	}
	payloadStart, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	payloadHash, payloadDigest, err := payloadVerifier(sig, h)
	if err != nil {
		return nil, err
	}
	headerHash, packageHash := crypto.SHA256.New(), crypto.SHA256.New()
	headerHash.Write(h.raw)
	packageHash.Write(h.raw)
	if _, err := io.Copy(io.MultiWriter(packageHash, payloadHash), r); err != nil {
		return nil, err
	}
	if hex.EncodeToString(payloadHash.Sum(nil)) != payloadDigest {
		return nil, ErrInvalidPackage
	}
	if _, err := r.Seek(payloadStart, io.SeekStart); err != nil {
		return nil, err
	}

	headerSignature, err := signOpenPGP(key, headerHash)
	if err != nil {
		return nil, err
	}
	packageSignature, err := signOpenPGP(key, packageHash)
	if err != nil {
		return nil, err
	}
	for _, tag := range []uint32{sigTagDSA, sigTagGPG, sigTagReservedSpace} {
		delete(sig.entries, tag)
	}
	sig.entries[sigTagRSA] = headerEntry{typ: typeBin, count: uint32(len(headerSignature)), data: headerSignature}
	sig.entries[sigTagPGP] = headerEntry{typ: typeBin, count: uint32(len(packageSignature)), data: packageSignature}

	return io.MultiReader(bytes.NewReader(lead), bytes.NewReader(sig.marshalSignature()), bytes.NewReader(h.raw), r), nil
}

func payloadVerifier(sig, h *header) (hash.Hash, string, error) {
	if newHash, ok := payloadDigestHashes[h.getUint(tagPayloadDigestAlgo)]; ok {
		if digest := h.getString(tagPayloadDigest); digest != "" {
			return newHash(), digest, nil
		}
	}
	if md5sum := sig.entries[sigTagMD5]; md5sum.typ == typeBin && len(md5sum.data) > 0 {
		hasher := md5.New()
		hasher.Write(h.raw)
		return hasher, hex.EncodeToString(md5sum.data), nil
	}
	return nil, "", ErrInvalidPackage
}

func signOpenPGP(key *packet.PrivateKey, digest hash.Hash) ([]byte, error) {
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

func (h *header) marshalSignature() []byte {
	tags := slices.DeleteFunc(slices.Sorted(maps.Keys(h.entries)), func(tag uint32) bool { return tag <= maxRegionTag })
	var index, store []byte
	for _, tag := range tags {
		entry := h.entries[tag]
		if size := typeSizes[entry.typ]; size > 1 {
			store = append(store, make([]byte, (size-len(store)%size)%size)...)
		}
		index = appendUint32s(index, tag, entry.typ, uint32(len(store)), entry.count)
		store = append(store, entry.data...)
	}

	count := uint32(len(tags) + 1)
	region := appendUint32s(nil, sigTagRegion, typeBin, uint32(len(store)), indexEntrySize)
	store = appendUint32s(store, sigTagRegion, typeBin, -count*indexEntrySize, indexEntrySize)
	out := appendUint32s(nil, headerMagic, 0, count, uint32(len(store)))
	out = append(append(append(out, region...), index...), store...)
	return append(out, make([]byte, signaturePadding(len(store)))...)
}

func appendUint32s(buf []byte, values ...uint32) []byte {
	for _, value := range values {
		buf = binary.BigEndian.AppendUint32(buf, value)
	}
	return buf
}

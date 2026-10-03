// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"slices"
	"strings"

	"gitea.dev/modules/util"
)

// https://rpm-software-management.github.io/rpm/manual/format_v4.html
// TODO: replace this reader with go-rpmutils once https://github.com/sassoftware/go-rpmutils/pull/44 is released

var ErrInvalidPackage = util.NewInvalidArgumentErrorf("invalid RPM package")

const (
	leadSize         = 96
	leadMagic        = 0xedabeedb
	headerMagic      = 0x8eade801
	headerIntroSize  = 16
	indexEntrySize   = 16
	maxHeaderEntries = 0xffff     // rpmlib's hdrchkTags
	maxHeaderData    = 0x0fffffff // rpmlib's hdrchkData
	maxRegionTag     = 63
)

const (
	typeNull = iota
	typeChar
	typeInt8
	typeInt16
	typeInt32
	typeInt64
	typeString
	typeBin
	typeStringArray
	typeI18NString
)

var typeSizes = map[uint32]int{typeNull: 0, typeChar: 1, typeInt8: 1, typeInt16: 2, typeInt32: 4, typeInt64: 8, typeBin: 1}

const (
	tagName            = 1000
	tagVersion         = 1001
	tagRelease         = 1002
	tagEpoch           = 1003
	tagSummary         = 1004
	tagDescription     = 1005
	tagBuildTime       = 1006
	tagBuildHost       = 1007
	tagSize            = 1009
	tagVendor          = 1011
	tagLicense         = 1014
	tagPackager        = 1015
	tagGroup           = 1016
	tagURL             = 1020
	tagArch            = 1022
	tagFileModes       = 1030
	tagFileMTimes      = 1034
	tagFileFlags       = 1037
	tagSourceRpm       = 1044
	tagProvideName     = 1047
	tagRequireFlags    = 1048
	tagRequireName     = 1049
	tagRequireVersion  = 1050
	tagConflictFlags   = 1053
	tagConflictName    = 1054
	tagConflictVersion = 1055
	tagChangelogTime   = 1080
	tagChangelogName   = 1081
	tagChangelogText   = 1082
	tagObsoleteName    = 1090
	tagProvideFlags    = 1112
	tagProvideVersion  = 1113
	tagObsoleteFlags   = 1114
	tagObsoleteVersion = 1115
	tagDirIndexes      = 1116
	tagBaseNames       = 1117
	tagDirNames        = 1118

	sigTagRegion        = 62
	sigTagDSA           = 267
	sigTagRSA           = 268
	sigTagSHA1          = 269
	sigTagSHA256        = 273
	sigTagPGP           = 1002
	sigTagGPG           = 1005
	sigTagPayloadSize   = 1007
	sigTagReservedSpace = 1008
)

type headerEntry struct {
	typ, count uint32
	data       []byte
}

type header struct {
	raw     []byte
	entries map[uint32]headerEntry
}

// readHeader validates the index like rpmlib's hdrblobVerifyInfo, so data is only allocated as it is read
func readHeader(r io.Reader) (*header, error) {
	intro := make([]byte, headerIntroSize)
	if _, err := io.ReadFull(r, intro); err != nil {
		return nil, err
	}
	indexCount, dataSize := binary.BigEndian.Uint32(intro[8:]), binary.BigEndian.Uint32(intro[12:])
	if binary.BigEndian.Uint32(intro) != headerMagic || indexCount > maxHeaderEntries || dataSize > maxHeaderData {
		return nil, ErrInvalidPackage
	}
	size := headerIntroSize + int64(indexCount)*indexEntrySize + int64(dataSize)
	buf := bytes.NewBuffer(intro)
	if _, err := buf.ReadFrom(io.LimitReader(r, size-headerIntroSize)); err != nil {
		return nil, err
	}
	if int64(buf.Len()) != size {
		return nil, io.ErrUnexpectedEOF
	}

	h := &header{raw: buf.Bytes(), entries: make(map[uint32]headerEntry, indexCount)}
	index := h.raw[headerIntroSize : headerIntroSize+indexCount*indexEntrySize]
	store := h.raw[len(index)+headerIntroSize:]
	end := uint32(0)
	for i := range indexCount {
		entry := index[i*indexEntrySize:]
		tag, typ := binary.BigEndian.Uint32(entry), binary.BigEndian.Uint32(entry[4:])
		offset, count := binary.BigEndian.Uint32(entry[8:]), binary.BigEndian.Uint32(entry[12:])
		if i == 0 && tag <= maxRegionTag {
			continue
		}
		if offset < end || offset > dataSize {
			return nil, ErrInvalidPackage
		}
		n, ok := dataLength(typ, count, store[offset:])
		if !ok {
			return nil, ErrInvalidPackage
		}
		end = offset + uint32(n)
		h.entries[tag] = headerEntry{typ: typ, count: count, data: store[offset:end]}
	}
	return h, nil
}

func dataLength(typ, count uint32, data []byte) (int, bool) {
	switch typ {
	case typeString, typeStringArray, typeI18NString:
		if int64(count) > int64(len(data)) {
			return 0, false
		}
		n := 0
		for range count {
			i := bytes.IndexByte(data[n:], 0)
			if i < 0 {
				return 0, false
			}
			n += i + 1
		}
		return n, true
	}
	size, ok := typeSizes[typ]
	n := int64(size) * int64(count)
	return int(n), ok && n <= int64(len(data))
}

// readHeaders reads the lead and both headers, rejecting a header that doesn't match the digest in the signature header
func readHeaders(r io.Reader) (lead []byte, sig, hdr *header, err error) {
	defer func() {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			err = ErrInvalidPackage
		}
	}()

	lead = make([]byte, leadSize)
	if _, err := io.ReadFull(r, lead); err != nil {
		return nil, nil, nil, err
	}
	if binary.BigEndian.Uint32(lead) != leadMagic {
		return nil, nil, nil, ErrInvalidPackage
	}
	if sig, err = readHeader(r); err != nil {
		return nil, nil, nil, err
	}
	if _, err := io.CopyN(io.Discard, r, int64(signaturePadding(len(sig.raw)))); err != nil {
		return nil, nil, nil, err
	}
	if hdr, err = readHeader(r); err != nil {
		return nil, nil, nil, err
	}

	if digest := sig.getString(sigTagSHA256); digest != "" {
		if sum := sha256.Sum256(hdr.raw); hex.EncodeToString(sum[:]) != digest {
			return nil, nil, nil, ErrInvalidPackage
		}
	} else if digest := sig.getString(sigTagSHA1); digest != "" {
		if sum := sha1.Sum(hdr.raw); hex.EncodeToString(sum[:]) != digest {
			return nil, nil, nil, ErrInvalidPackage
		}
	}
	return lead, sig, hdr, nil
}

// signaturePadding aligns the header following the signature header to 8 bytes
func signaturePadding(size int) int {
	return (8 - size%8) % 8
}

func (h *header) getStrings(tag uint32) []string {
	e, ok := h.entries[tag]
	if !ok || e.count == 0 || (e.typ != typeString && e.typ != typeStringArray && e.typ != typeI18NString) {
		return nil
	}
	return strings.Split(string(e.data[:len(e.data)-1]), "\x00")
}

func (h *header) getString(tag uint32) string {
	if values := h.getStrings(tag); len(values) > 0 {
		return values[0]
	}
	return ""
}

func (h *header) getUints(tag uint32) []uint64 {
	e, ok := h.entries[tag]
	if !ok || e.typ < typeChar || e.typ > typeInt64 {
		return nil
	}
	values := make([]uint64, 0, e.count)
	for chunk := range slices.Chunk(e.data, typeSizes[e.typ]) {
		switch len(chunk) {
		case 1:
			values = append(values, uint64(chunk[0]))
		case 2:
			values = append(values, uint64(binary.BigEndian.Uint16(chunk)))
		case 4:
			values = append(values, uint64(binary.BigEndian.Uint32(chunk)))
		default:
			values = append(values, binary.BigEndian.Uint64(chunk))
		}
	}
	return values
}

func (h *header) getUint(tag uint32) uint64 {
	if values := h.getUints(tag); len(values) > 0 {
		return values[0]
	}
	return 0
}

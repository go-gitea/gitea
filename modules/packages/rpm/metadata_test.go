// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"io"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePackage(t *testing.T) {
	base64RpmsignSignedPackage := `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
OvSXaRmpzyTLHvsXg/M9EcwbAAA=`
	rpmPackageContent, err := base64.StdEncoding.DecodeString(base64RpmsignSignedPackage)
	assert.NoError(t, err)

	zr, err := gzip.NewReader(bytes.NewReader(rpmPackageContent))
	assert.NoError(t, err)
	decompressed, err := io.ReadAll(zr)
	assert.NoError(t, err)

	p, err := ParsePackage(bytes.NewReader(decompressed))
	assert.NotNil(t, p)
	assert.NoError(t, err)

	assert.Equal(t, "gitea-test", p.Name)
	assert.Equal(t, "1.0.2-1", p.Version)
	assert.NotNil(t, p.VersionMetadata)
	assert.NotNil(t, p.FileMetadata)

	assert.Equal(t, "MIT", p.VersionMetadata.License)
	assert.Equal(t, "https://gitea.io", p.VersionMetadata.ProjectURL)
	assert.Equal(t, "RPM package summary", p.VersionMetadata.Summary)
	assert.Equal(t, "RPM package description", p.VersionMetadata.Description)

	assert.Equal(t, "x86_64", p.FileMetadata.Architecture)
	assert.Equal(t, "0", p.FileMetadata.Epoch)
	assert.Equal(t, "1.0.2", p.FileMetadata.Version)
	assert.Equal(t, "1", p.FileMetadata.Release)
	assert.Empty(t, p.FileMetadata.Vendor)
	assert.Equal(t, "KN4CK3R", p.FileMetadata.Packager)
	assert.Equal(t, "gitea-test-1.0.2-1.src.rpm", p.FileMetadata.SourceRpm)
	assert.Equal(t, "e44b1687d04b", p.FileMetadata.BuildHost)
	assert.EqualValues(t, 1678225964, p.FileMetadata.BuildTime)
	assert.EqualValues(t, 1678225964, p.FileMetadata.FileTime)
	assert.EqualValues(t, 13, p.FileMetadata.InstalledSize)
	assert.EqualValues(t, 272, p.FileMetadata.ArchiveSize)
	assert.Empty(t, p.FileMetadata.Conflicts)
	assert.Empty(t, p.FileMetadata.Obsoletes)

	assert.ElementsMatch(
		t,
		[]*Entry{
			{
				Name:    "gitea-test",
				Flags:   "EQ",
				Version: "1.0.2",
				Epoch:   "0",
				Release: "1",
			},
			{
				Name:    "gitea-test(x86-64)",
				Flags:   "EQ",
				Version: "1.0.2",
				Epoch:   "0",
				Release: "1",
			},
		},
		p.FileMetadata.Provides,
	)
	assert.ElementsMatch(
		t,
		[]*Entry{
			{
				Name: "/bin/sh",
			},
			{
				Name: "/bin/sh",
			},
			{
				Name: "/bin/sh",
			},
			{
				Name:    "rpmlib(CompressedFileNames)",
				Flags:   "LE",
				Version: "3.0.4",
				Epoch:   "0",
				Release: "1",
			},
			{
				Name:    "rpmlib(FileDigests)",
				Flags:   "LE",
				Version: "4.6.0",
				Epoch:   "0",
				Release: "1",
			},
			{
				Name:    "rpmlib(PayloadFilesHavePrefix)",
				Flags:   "LE",
				Version: "4.0",
				Epoch:   "0",
				Release: "1",
			},
			{
				Name:    "rpmlib(PayloadIsXz)",
				Flags:   "LE",
				Version: "5.2",
				Epoch:   "0",
				Release: "1",
			},
		},
		p.FileMetadata.Requires,
	)
	assert.ElementsMatch(
		t,
		[]*File{
			{
				Path:         "/usr/local/bin/hello",
				IsExecutable: true,
			},
		},
		p.FileMetadata.Files,
	)
	assert.ElementsMatch(
		t,
		[]*Changelog{
			{
				Author: "KN4CK3R <dummy@gitea.io>",
				Date:   1678276800,
				Text:   "- Changelog message.",
			},
		},
		p.FileMetadata.Changelogs,
	)

	_, sig, h, err := readHeaders(bytes.NewReader(decompressed))
	require.NoError(t, err)
	headerStart := leadSize + len(sig.raw) + signaturePadding(len(sig.raw))

	t.Run("RejectsInvalidHeaders", func(t *testing.T) {
		for _, corrupt := range []func([]byte){
			func(b []byte) { binary.BigEndian.PutUint32(b[leadSize+12:], maxSignatureData+1) },
			func(b []byte) { b[headerStart+len(h.raw)-1] ^= 1 },
			func(b []byte) {
				binary.BigEndian.PutUint32(b[leadSize+headerIntroSize+3*indexEntrySize+8:], maxHeaderData)
			},
			func(b []byte) {
				copy(b[leadSize+headerIntroSize+4*indexEntrySize+8:][:4], b[leadSize+headerIntroSize+3*indexEntrySize+8:])
			},
		} {
			content := bytes.Clone(decompressed)
			corrupt(content)
			_, err := ParsePackage(bytes.NewReader(content))
			assert.ErrorIs(t, err, ErrInvalidPackage)
		}
	})

	t.Run("RejectsOversizedArrays", func(t *testing.T) {
		for _, typ := range []uint32{typeInt8, typeStringArray} {
			const count = 1024 * 1024
			content := appendUint32s(nil, headerMagic, 0, 1, count)
			content = appendUint32s(content, tagFileMTimes, typ, 0, count)
			content = append(content, make([]byte, count)...)
			_, err := readHeader(bytes.NewReader(content), maxHeaderEntries, maxHeaderData)
			assert.ErrorIs(t, err, ErrInvalidPackage)
		}
	})

	t.Run("SignPackage", func(t *testing.T) {
		assert.Equal(t, decompressed[leadSize:headerStart], sig.marshalSignature())

		entity, err := openpgp.NewEntity("", "", "", &packet.Config{RSABits: 1024})
		require.NoError(t, err)
		signed, err := SignPackage(bytes.NewReader(decompressed), entity.PrivateKey)
		require.NoError(t, err)
		content, err := io.ReadAll(signed)
		require.NoError(t, err)

		_, signedSig, signedHeader, err := readHeaders(bytes.NewReader(content))
		require.NoError(t, err)
		_, err = openpgp.CheckDetachedSignature(openpgp.EntityList{entity}, bytes.NewReader(signedHeader.raw), bytes.NewReader(signedSig.entries[sigTagRSA].data), nil)
		assert.NoError(t, err)
		_, err = openpgp.CheckDetachedSignature(openpgp.EntityList{entity}, bytes.NewReader(decompressed[headerStart:]), bytes.NewReader(signedSig.entries[sigTagPGP].data), nil)
		assert.NoError(t, err)
		assert.Equal(t, decompressed[:leadSize], content[:leadSize])
		assert.Equal(t, decompressed[headerStart:], content[leadSize+len(signedSig.raw)+signaturePadding(len(signedSig.raw)):])

		signedPackage, err := ParsePackage(bytes.NewReader(content))
		require.NoError(t, err)
		assert.Equal(t, p, signedPackage)

		corruptPayload := bytes.Clone(decompressed)
		corruptPayload[len(corruptPayload)-1] ^= 1
		_, err = SignPackage(bytes.NewReader(corruptPayload), entity.PrivateKey)
		assert.ErrorIs(t, err, ErrInvalidPackage)
	})
}

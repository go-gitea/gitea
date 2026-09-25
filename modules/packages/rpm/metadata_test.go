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
	base64RpmPackageContent := `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`
	rpmPackageContent, err := base64.StdEncoding.DecodeString(base64RpmPackageContent)
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
			func(b []byte) { binary.BigEndian.PutUint32(b[leadSize+12:], maxHeaderData) },
			func(b []byte) { binary.BigEndian.PutUint32(b[leadSize+12:], maxHeaderData+1) },
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

	t.Run("SignPackage", func(t *testing.T) {
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
	})
}

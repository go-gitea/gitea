// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gitea.dev/modules/util"

	"github.com/sassoftware/go-rpmutils"
	"github.com/stretchr/testify/assert"
)

func TestParsePackage(t *testing.T) {
	base64RpmPackageContent := `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`
	rpmPackageContent, err := base64.StdEncoding.DecodeString(base64RpmPackageContent)
	assert.NoError(t, err)

	zr, err := gzip.NewReader(bytes.NewReader(rpmPackageContent))
	assert.NoError(t, err)

	p, err := ParsePackage(zr)
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
}

func TestParsePackageLimits(t *testing.T) {
	type tag struct {
		id, typ, count int32
		data           []byte
	}
	stringTag := func(id int32, values ...string) tag {
		return tag{id, rpmutils.RPM_STRING_ARRAY_TYPE, int32(len(values)), []byte(strings.Join(values, "\x00") + "\x00")}
	}
	writeHeader := func(buf *bytes.Buffer, tags []tag, declaredSize uint32) {
		var data bytes.Buffer
		for _, t := range tags {
			data.Write(t.data)
		}
		size := uint32(data.Len())
		if declaredSize != 0 {
			size = declaredSize
		}
		_ = binary.Write(buf, binary.BigEndian, []uint32{0x8eade801, 0, uint32(len(tags)), size})
		offset := int32(0)
		for _, t := range tags {
			_ = binary.Write(buf, binary.BigEndian, []int32{t.id, t.typ, offset, t.count})
			offset += int32(len(t.data))
		}
		buf.Write(data.Bytes())
	}
	createPackage := func(tags []tag, declaredSize uint32) *bytes.Buffer {
		var buf bytes.Buffer
		buf.Write([]byte{0xed, 0xab, 0xee, 0xdb})
		buf.Write(make([]byte, 92))
		writeHeader(&buf, nil, 0)
		writeHeader(&buf, tags, declaredSize)
		return &buf
	}
	createRequires := func(n int) []tag {
		flags := make([]byte, 4*n)
		return []tag{
			stringTag(rpmutils.NAME, "test"),
			stringTag(rpmutils.VERSION, "1.0"),
			stringTag(rpmutils.RELEASE, "1"),
			stringTag(rpmutils.ARCH, "noarch"),
			stringTag(rpmutils.REQUIRENAME, slices.Repeat([]string{"dep"}, n)...),
			stringTag(rpmutils.REQUIREVERSION, slices.Repeat([]string{""}, n)...),
			{rpmutils.REQUIREFLAGS, rpmutils.RPM_INT32_TYPE, int32(n), flags},
		}
	}

	t.Run("Valid", func(t *testing.T) {
		p, err := ParsePackage(createPackage(createRequires(2), 0))
		assert.NoError(t, err)
		assert.Len(t, p.FileMetadata.Requires, 2)
	})

	t.Run("DeclaredHeaderSize", func(t *testing.T) {
		// above the header cap, and below it but beyond the uploaded data
		for _, declaredSize := range []uint32{1024 * 1024 * 1024, 32 * 1024 * 1024} {
			data := createPackage(nil, declaredSize)
			assert.Equal(t, 128, data.Len())

			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			p, err := ParsePackage(data)
			runtime.ReadMemStats(&after)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, util.ErrInvalidArgument)
			assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(1024*1024))
		}
	})

	t.Run("TooManyEntries", func(t *testing.T) {
		p, err := ParsePackage(createPackage(createRequires(maxDependencyEntries+1), 0))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
	})
}

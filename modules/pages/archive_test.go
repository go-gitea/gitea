// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testArchiveEntry struct {
	header *tar.Header
	body   []byte
}

func testLimits() Limits {
	return Limits{MaxCompressedBytes: 1024 * 1024, MaxTarBytes: 1024 * 1024, MaxSiteBytes: 64 * 1024, MaxFileBytes: 32 * 1024, MaxEntries: 32, MaxPathBytes: 128, MaxPathDepth: 8, MaxSymlinkDepth: 4}
}

func testArchive(t *testing.T, entries ...testArchiveEntry) []byte {
	t.Helper()
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		require.NoError(t, tw.WriteHeader(entry.header))
		if entry.header.Typeflag == tar.TypeReg {
			require.Len(t, entry.body, int(entry.header.Size))
			_, err := tw.Write(entry.body)
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return data.Bytes()
}

func testFile(name, body string) testArchiveEntry {
	return testArchiveEntry{header: &tar.Header{Name: name, Typeflag: tar.TypeReg, Size: int64(len(body))}, body: []byte(body)}
}

func testHeader(header *tar.Header) testArchiveEntry {
	return testArchiveEntry{header: header}
}

func TestValidateAcceptsMissingInRootSymlink(t *testing.T) {
	data := testArchive(t,
		testFile("index.html", "page"),
		testHeader(&tar.Header{Name: "latest", Typeflag: tar.TypeSymlink, Linkname: "releases/current"}),
	)
	digest, err := ValidateArchive(t.Context(), bytes.NewReader(data), testLimits())
	require.NoError(t, err)
	want := sha256.Sum256(data)
	assert.Equal(t, hex.EncodeToString(want[:]), digest)
}

func TestValidateAcceptsIndexlessWebsite(t *testing.T) {
	data := testArchive(t, testFile("assets/app.js", "page"))
	_, err := ValidateArchive(t.Context(), bytes.NewReader(data), testLimits())
	require.NoError(t, err)
}

func TestValidateRejectsUnsafePathsAndTypes(t *testing.T) {
	tests := []struct {
		name  string
		entry testArchiveEntry
		want  error
	}{
		{"traversal", testFile("../index.html", "page"), ErrArchivePath},
		{"absolute", testFile("/index.html", "page"), ErrArchivePath},
		{"absolute root", testHeader(&tar.Header{Name: "/", Typeflag: tar.TypeDir}), ErrArchivePath},
		{"escaping symlink", testHeader(&tar.Header{Name: "asset", Typeflag: tar.TypeSymlink, Linkname: "../../secret"}), ErrArchiveSymlink},
		{"device", testHeader(&tar.Header{Name: "device", Typeflag: tar.TypeBlock}), ErrArchiveEntry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateArchive(t.Context(), bytes.NewReader(testArchive(t, tt.entry, testFile("index.html", "page"))), testLimits())
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestValidateRejectsFilesystemPathCollisions(t *testing.T) {
	for _, names := range [][2]string{
		{"asset.js", "asset.js"},
		{"README", "readme"},
		{"\u00e9.html", "e\u0301.html"},
	} {
		t.Run(names[0]+"_"+names[1], func(t *testing.T) {
			_, err := ValidateArchive(t.Context(), bytes.NewReader(testArchive(t,
				testFile(names[0], "first"),
				testFile(names[1], "second"),
			)), testLimits())
			require.ErrorIs(t, err, ErrArchivePath)
		})
	}
}

func TestValidateRejectsSymlinkParentLoopAndDepth(t *testing.T) {
	t.Run("symlink parent", func(t *testing.T) {
		_, err := ValidateArchive(t.Context(), bytes.NewReader(testArchive(t,
			testHeader(&tar.Header{Name: "assets", Typeflag: tar.TypeSymlink, Linkname: "static"}),
			testFile("assets/app.js", "page"),
		)), testLimits())
		require.ErrorIs(t, err, ErrArchiveEntry)
	})

	t.Run("loop", func(t *testing.T) {
		_, err := ValidateArchive(t.Context(), bytes.NewReader(testArchive(t,
			testFile("index.html", "page"),
			testHeader(&tar.Header{Name: "a", Typeflag: tar.TypeSymlink, Linkname: "b"}),
			testHeader(&tar.Header{Name: "b", Typeflag: tar.TypeSymlink, Linkname: "a"}),
		)), testLimits())
		require.ErrorIs(t, err, ErrArchiveSymlink)
	})

	t.Run("depth", func(t *testing.T) {
		limits := testLimits()
		limits.MaxSymlinkDepth = 1
		_, err := ValidateArchive(t.Context(), bytes.NewReader(testArchive(t,
			testFile("index.html", "page"),
			testHeader(&tar.Header{Name: "a", Typeflag: tar.TypeSymlink, Linkname: "b"}),
			testHeader(&tar.Header{Name: "b", Typeflag: tar.TypeSymlink, Linkname: "c"}),
			testHeader(&tar.Header{Name: "c", Typeflag: tar.TypeSymlink, Linkname: "missing"}),
		)), limits)
		require.ErrorIs(t, err, ErrArchiveSymlink)
	})
}

func TestValidateRejectsArchiveTrailingData(t *testing.T) {
	valid := testArchive(t, testFile("index.html", "page"))

	t.Run("concatenated gzip member", func(t *testing.T) {
		_, err := ValidateArchive(t.Context(), bytes.NewReader(append(append([]byte{}, valid...), valid...)), testLimits())
		require.ErrorIs(t, err, ErrInvalidArchive)
	})

	t.Run("data after gzip", func(t *testing.T) {
		_, err := ValidateArchive(t.Context(), bytes.NewReader(append(append([]byte{}, valid...), 1)), testLimits())
		require.ErrorIs(t, err, ErrInvalidArchive)
	})

	t.Run("non-zero data after TAR", func(t *testing.T) {
		var tarData bytes.Buffer
		tw := tar.NewWriter(&tarData)
		entry := testFile("index.html", "page")
		require.NoError(t, tw.WriteHeader(entry.header))
		_, err := tw.Write(entry.body)
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		tarData.WriteByte(1)

		var compressed bytes.Buffer
		gz := gzip.NewWriter(&compressed)
		_, err = gz.Write(tarData.Bytes())
		require.NoError(t, err)
		require.NoError(t, gz.Close())

		_, err = ValidateArchive(t.Context(), bytes.NewReader(compressed.Bytes()), testLimits())
		require.ErrorIs(t, err, ErrInvalidArchive)
	})
}

func TestValidateArchiveLimitsIncludeBoundary(t *testing.T) {
	t.Run("file size", func(t *testing.T) {
		limits := testLimits()
		limits.MaxFileBytes = 4
		limits.MaxSiteBytes = 4
		_, err := ValidateArchive(t.Context(), bytes.NewReader(testArchive(t, testFile("index.html", "page"))), limits)
		require.NoError(t, err)

		_, err = ValidateArchive(t.Context(), bytes.NewReader(testArchive(t, testFile("index.html", "pages"))), limits)
		require.ErrorIs(t, err, ErrArchiveTooLarge)
	})

	t.Run("entry count", func(t *testing.T) {
		limits := testLimits()
		limits.MaxEntries = 2
		_, err := ValidateArchive(t.Context(), bytes.NewReader(testArchive(t,
			testHeader(&tar.Header{Name: "assets", Typeflag: tar.TypeDir}),
			testFile("index.html", "page"),
		)), limits)
		require.NoError(t, err)

		_, err = ValidateArchive(t.Context(), bytes.NewReader(testArchive(t,
			testHeader(&tar.Header{Name: "assets", Typeflag: tar.TypeDir}),
			testFile("index.html", "page"),
			testFile("404.html", "page"),
		)), limits)
		require.ErrorIs(t, err, ErrArchiveTooLarge)
	})
}

func TestValidateArchiveHonorsCancellationWhileReading(t *testing.T) {
	makeArchive := func(t *testing.T, bodyBytes, tailBytes int) []byte {
		t.Helper()
		var tarData bytes.Buffer
		tw := tar.NewWriter(&tarData)
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: "index.html", Typeflag: tar.TypeReg, Size: int64(bodyBytes)}))
		_, err := tw.Write(bytes.Repeat([]byte{'a'}, bodyBytes))
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		tarData.Write(make([]byte, tailBytes))

		var compressed bytes.Buffer
		gz, err := gzip.NewWriterLevel(&compressed, gzip.NoCompression)
		require.NoError(t, err)
		_, err = gz.Write(tarData.Bytes())
		require.NoError(t, err)
		require.NoError(t, gz.Close())
		return compressed.Bytes()
	}

	for _, tt := range []struct {
		name      string
		bodyBytes int
		tailBytes int
		cancelAt  func(int) int
	}{
		{name: "file body", bodyBytes: 128 * 1024, cancelAt: func(size int) int { return size / 2 }},
		{name: "TAR tail", bodyBytes: 4, tailBytes: 128 * 1024, cancelAt: func(size int) int { return size - 64*1024 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := makeArchive(t, tt.bodyBytes, tt.tailBytes)
			ctx, cancel := context.WithCancel(t.Context())
			reader := &cancelAfterReader{reader: bytes.NewReader(data), cancel: cancel, remaining: tt.cancelAt(len(data))}
			limits := testLimits()
			limits.MaxCompressedBytes = int64(len(data) + 1)
			limits.MaxTarBytes = int64(tt.bodyBytes + tt.tailBytes + 4*1024)
			limits.MaxSiteBytes = int64(tt.bodyBytes + 1)
			limits.MaxFileBytes = int64(tt.bodyBytes + 1)

			_, err := ValidateArchive(ctx, reader, limits)
			require.ErrorIs(t, err, context.Canceled)
			assert.Less(t, reader.read, len(data))
		})
	}
}

type cancelAfterReader struct {
	reader    io.Reader
	cancel    context.CancelFunc
	remaining int
	read      int
}

func (r *cancelAfterReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		r.cancel()
	}
	if r.remaining > 0 && len(p) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.read += n
	r.remaining -= n
	return n, err
}

func TestPackDirectoryProducesSafeDeterministicArchive(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("page"), 0o755))
	require.NoError(t, os.Symlink("missing/page", filepath.Join(root, "latest")))
	var first, second bytes.Buffer
	require.NoError(t, PackDirectory(context.Background(), root, &first, testLimits()))
	require.NoError(t, PackDirectory(context.Background(), root, &second, testLimits()))
	assert.Equal(t, first.Bytes(), second.Bytes())

	_, err := ValidateArchive(t.Context(), bytes.NewReader(first.Bytes()), testLimits())
	require.NoError(t, err)

	gz, err := gzip.NewReader(bytes.NewReader(first.Bytes()))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Typeflag == tar.TypeReg && hdr.Name == "index.html" {
			assert.Zero(t, hdr.Mode&0o111, "regular file retained execute bits")
		}
	}
}

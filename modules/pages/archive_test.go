// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func testLimits() Limits {
	return Limits{MaxCompressedBytes: 1024 * 1024, MaxTarBytes: 1024 * 1024, MaxSiteBytes: 64 * 1024, MaxFileBytes: 32 * 1024, MaxEntries: 32, MaxPathBytes: 128, MaxPathDepth: 8, MaxSymlinkDepth: 4}
}

func testArchive(t *testing.T, entries ...*tar.Header) []byte {
	t.Helper()
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tw := tar.NewWriter(gz)
	for _, hdr := range entries {
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := io.WriteString(tw, "page"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestValidateAcceptsMissingInRootSymlink(t *testing.T) {
	data := testArchive(t,
		&tar.Header{Name: "index.html", Typeflag: tar.TypeReg, Size: 4},
		&tar.Header{Name: "latest", Typeflag: tar.TypeSymlink, Linkname: "releases/current"},
	)
	manifest, err := ValidateArchive(t.Context(), bytes.NewReader(data), testLimits())
	if err != nil {
		t.Fatalf("ValidateArchive() error = %v", err)
	}
	if manifest.SiteBytes != 4 || manifest.Entries != 2 || manifest.SHA256Hex == "" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
}

func TestValidateRejectsUnsafePathsAndTypes(t *testing.T) {
	tests := []struct {
		name string
		hdr  *tar.Header
		want error
	}{
		{"traversal", &tar.Header{Name: "../index.html", Typeflag: tar.TypeReg, Size: 4}, ErrArchivePath},
		{"absolute", &tar.Header{Name: "/index.html", Typeflag: tar.TypeReg, Size: 4}, ErrArchivePath},
		{"absolute root", &tar.Header{Name: "/", Typeflag: tar.TypeDir}, ErrArchivePath},
		{"escaping symlink", &tar.Header{Name: "asset", Typeflag: tar.TypeSymlink, Linkname: "../../secret"}, ErrArchiveSymlink},
		{"device", &tar.Header{Name: "device", Typeflag: tar.TypeBlock}, ErrArchiveEntry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateArchive(t.Context(), bytes.NewReader(testArchive(t, tt.hdr, &tar.Header{Name: "index.html", Typeflag: tar.TypeReg, Size: 4})), testLimits())
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateArchive() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPackDirectoryProducesSafeDeterministicArchive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("page"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing/page", filepath.Join(root, "latest")); err != nil {
		t.Fatal(err)
	}
	var first, second bytes.Buffer
	if err := PackDirectory(context.Background(), root, &first, testLimits()); err != nil {
		t.Fatal(err)
	}
	if err := PackDirectory(context.Background(), root, &second, testLimits()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("PackDirectory output is not deterministic")
	}
	manifest, err := ValidateArchive(t.Context(), bytes.NewReader(first.Bytes()), testLimits())
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SiteBytes != 4 {
		t.Fatalf("SiteBytes = %d, want 4", manifest.SiteBytes)
	}
	gz, err := gzip.NewReader(bytes.NewReader(first.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg && hdr.Name == "index.html" && hdr.Mode&0o111 != 0 {
			t.Fatalf("regular file retained execute bits: %#o", hdr.Mode)
		}
	}
}

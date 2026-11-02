// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cran

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"testing"

	"gitea.dev/modules/packages"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	packageName    = "gitea"
	packageVersion = "1.0.1"
	author         = "KN4CK3R"
	description    = "Package Description"
	projectURL     = "https://gitea.io"
	license        = "GPL (>= 2)"
)

func createDescription(name, version string) *bytes.Buffer {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "Package:", name)
	fmt.Fprintln(&buf, "Version:", version)
	fmt.Fprintln(&buf, "Description:", "Package\n\n  Description")
	fmt.Fprintln(&buf, "URL:", projectURL)
	fmt.Fprintln(&buf, "Imports: abc,\n123")
	fmt.Fprintln(&buf, "NeedsCompilation: yes")
	fmt.Fprintln(&buf, "License:", license)
	fmt.Fprintln(&buf, "Author:", author)
	return &buf
}

func TestParsePackage(t *testing.T) {
	for _, archive := range []struct {
		extension string
		create    func(string, []byte) *bytes.Reader
	}{
		{".tar.gz", func(filename string, content []byte) *bytes.Reader {
			var buf bytes.Buffer
			gw := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gw)
			hdr := &tar.Header{
				Name: filename,
				Mode: 0o600,
				Size: int64(len(content)),
			}
			tw.WriteHeader(hdr)
			tw.Write(content)
			tw.Close()
			gw.Close()
			return bytes.NewReader(buf.Bytes())
		}},
		{".zip", func(filename string, content []byte) *bytes.Reader {
			var buf bytes.Buffer
			archive := zip.NewWriter(&buf)
			w, _ := archive.Create(filename)
			w.Write(content)
			archive.Close()
			return bytes.NewReader(buf.Bytes())
		}},
	} {
		t.Run(archive.extension, func(t *testing.T) {
			t.Run("MissingDescriptionFile", func(t *testing.T) {
				buf := archive.create(
					"dummy.txt",
					[]byte{},
				)

				p, err := ParsePackage(buf, buf.Size())
				assert.Nil(t, p)
				assert.ErrorIs(t, err, ErrMissingDescriptionFile)
			})

			t.Run("Valid", func(t *testing.T) {
				buf := archive.create(
					"package/DESCRIPTION",
					createDescription(packageName, packageVersion).Bytes(),
				)

				p, err := ParsePackage(buf, buf.Size())
				assert.NotNil(t, p)
				assert.NoError(t, err)

				assert.Equal(t, packageName, p.Name)
				assert.Equal(t, packageVersion, p.Version)
			})
		})
	}
}

func TestParseDescription(t *testing.T) {
	t.Run("InvalidName", func(t *testing.T) {
		for _, name := range []string{"123abc", "ab-cd", "ab cd", "ab/cd"} {
			p, err := ParseDescription(createDescription(name, packageVersion))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidName)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		for _, version := range []string{"1", "1 0", "1.", "1.0.", "1-", "1-0-"} {
			p, err := ParseDescription(createDescription(packageName, version))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidVersion)
		}
	})

	t.Run("ValidVersionManyComponents", func(t *testing.T) {
		for _, version := range []string{"0.3.4.0.2", "1.2.3.4.5", "1-2-3-4-5"} {
			p, err := ParseDescription(createDescription(packageName, version))
			assert.NoError(t, err)
			assert.NotNil(t, p)
			assert.Equal(t, version, p.Version)
		}
	})

	t.Run("Valid", func(t *testing.T) {
		p, err := ParseDescription(createDescription(packageName, packageVersion))
		assert.NoError(t, err)
		assert.NotNil(t, p)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, description, p.Metadata.Description)
		assert.ElementsMatch(t, []string{projectURL}, p.Metadata.ProjectURL)
		assert.ElementsMatch(t, []string{author}, p.Metadata.Authors)
		assert.Equal(t, license, p.Metadata.License)
		assert.ElementsMatch(t, []string{"abc", "123"}, p.Metadata.Imports)
		assert.True(t, p.Metadata.NeedsCompilation)
	})
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b     string
		expected int
	}{
		{"1.0.3", "1.0.2", 1},
		{"0.9.10", "0.9.9", 1},
		{"2.5.2", "3.0.2", -1},
		{"1.0-1", "1.0.0", 1},
		{"1.0-2", "1.0.2", 0},
		{"1.0", "1.0.0", -1},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, CompareVersions(c.a, c.b), "%s vs %s", c.a, c.b)
	}
}

func TestParsePackageArchiveTooLarge(t *testing.T) {
	var buf bytes.Buffer
	gw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	require.NoError(t, err)
	tw := tar.NewWriter(gw)

	// a member which inflates past the limit, placed before the description file
	padding := int64(maxArchiveSize) + 1 // one byte past what the parser allows
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "padding.bin", Mode: 0o600, Size: padding}))
	chunk := make([]byte, 1<<20)
	for remaining := padding; remaining > 0; remaining -= int64(len(chunk)) {
		_, err := tw.Write(chunk[:min(remaining, int64(len(chunk)))])
		require.NoError(t, err)
	}

	content := createDescription(packageName, packageVersion).Bytes()
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "gitea/DESCRIPTION", Mode: 0o600, Size: int64(len(content))}))
	_, err = tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	_, err = ParsePackage(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	assert.ErrorIs(t, err, packages.ErrContentTooLarge)
}

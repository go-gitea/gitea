// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/ulikunitz/xz"
)

const (
	packageName         = "gitea"
	packageVersion      = "0:1.0.1-te~st"
	packageArchitecture = "amd64"
	packageAuthor       = "KN4CK3R"
	description         = "Description with multiple lines."
	projectURL          = "https://gitea.io"
)

func TestParsePackage(t *testing.T) {
	createArchive := func(files map[string][]byte) io.Reader {
		var buf bytes.Buffer
		aw := ar.NewWriter(&buf)
		aw.WriteGlobalHeader()
		for filename, content := range files {
			hdr := &ar.Header{
				Name: filename,
				Mode: 0o600,
				Size: int64(len(content)),
			}
			aw.WriteHeader(hdr)
			aw.Write(content)
		}
		return &buf
	}

	t.Run("MissingControlFile", func(t *testing.T) {
		data := createArchive(map[string][]byte{"dummy.txt": {}})

		p, err := ParsePackage(data)
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrMissingControlFile)
	})

	t.Run("Compression", func(t *testing.T) {
		t.Run("Unsupported", func(t *testing.T) {
			data := createArchive(map[string][]byte{"control.tar.foo": {}})

			p, err := ParsePackage(data)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrUnsupportedCompression)
		})

		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		tw.WriteHeader(&tar.Header{
			Name: "control",
			Mode: 0o600,
			Size: 50,
		})
		tw.Write([]byte("Package: gitea\nVersion: 1.0.0\nArchitecture: amd64\n"))
		tw.Close()

		cases := []struct {
			Extension     string
			WriterFactory func(io.Writer) io.WriteCloser
		}{
			{
				Extension: "",
				WriterFactory: func(w io.Writer) io.WriteCloser {
					return nopCloser{w}
				},
			},
			{
				Extension: ".gz",
				WriterFactory: func(w io.Writer) io.WriteCloser {
					return gzip.NewWriter(w)
				},
			},
			{
				Extension: ".xz",
				WriterFactory: func(w io.Writer) io.WriteCloser {
					xw, _ := xz.NewWriter(w)
					return xw
				},
			},
			{
				Extension: ".zst",
				WriterFactory: func(w io.Writer) io.WriteCloser {
					zw, _ := zstd.NewWriter(w)
					return zw
				},
			},
		}

		for _, c := range cases {
			t.Run(c.Extension, func(t *testing.T) {
				var cbuf bytes.Buffer
				w := c.WriterFactory(&cbuf)
				w.Write(buf.Bytes())
				w.Close()

				data := createArchive(map[string][]byte{"control.tar" + c.Extension: cbuf.Bytes()})

				p, err := ParsePackage(data)
				assert.NotNil(t, p)
				assert.NoError(t, err)
				assert.Equal(t, "gitea", p.Name)

				t.Run("TrailingSlash", func(t *testing.T) {
					data := createArchive(map[string][]byte{"control.tar" + c.Extension + "/": cbuf.Bytes()})

					p, err := ParsePackage(data)
					assert.NotNil(t, p)
					assert.NoError(t, err)
					assert.Equal(t, "gitea", p.Name)
				})
			})
		}
	})
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

func TestParseControlFile(t *testing.T) {
	buildContent := func(name, version, architecture string) *bytes.Buffer {
		var buf bytes.Buffer
		buf.WriteString("Package: " + name + "\nVersion: " + version + "\nArchitecture: " + architecture + "\nMaintainer: " + packageAuthor + " <kn4ck3r@gitea.io>\nHomepage: " + projectURL + "\nDepends: a,\n b\nDescription: Description\n with multiple\n lines.")
		return &buf
	}

	t.Run("InvalidName", func(t *testing.T) {
		for _, name := range []string{"", "-cd"} {
			p, err := ParseControlFile(buildContent(name, packageVersion, packageArchitecture))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidName)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		for _, version := range []string{"", "1-", ":1.0", "1_0"} {
			p, err := ParseControlFile(buildContent(packageName, version, packageArchitecture))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, ErrInvalidVersion)
		}
	})

	t.Run("InvalidArchitecture", func(t *testing.T) {
		p, err := ParseControlFile(buildContent(packageName, packageVersion, ""))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidArchitecture)
	})

	t.Run("Valid", func(t *testing.T) {
		content := buildContent(packageName, packageVersion, packageArchitecture)
		full := content.String()

		p, err := ParseControlFile(content)
		assert.NoError(t, err)
		assert.NotNil(t, p)

		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, packageArchitecture, p.Architecture)
		assert.Equal(t, description, p.Metadata.Description)
		assert.Equal(t, projectURL, p.Metadata.ProjectURL)
		assert.Equal(t, packageAuthor, p.Metadata.Maintainer)
		assert.Equal(t, []string{"a", "b"}, p.Metadata.Dependencies)
		assert.Equal(t, full, p.Control)
	})
}

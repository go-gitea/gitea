// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"testing"

	"gitea.dev/modules/util"
	"gitea.dev/modules/zstd"

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
	member := func(name, content string) string {
		return fmt.Sprintf("%-48s%-10d`\n%s%s", name, len(content), content, strings.Repeat("\n", len(content)%2))
	}
	debianBinary := member("debian-binary", "2.0\n")

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	tw.WriteHeader(&tar.Header{
		Name: "control",
		Mode: 0o600,
		Size: 50,
	})
	tw.Write([]byte("Package: gitea\nVersion: 1.0.0\nArchitecture: amd64\n"))
	tw.Close()

	t.Run("Invalid", func(t *testing.T) {
		for data, expected := range map[string]error{
			"!<ARCH>\n" + debianBinary:                                                GlobalVars().ErrInvalidArchive,
			arMagic + debianBinary[:20]:                                               GlobalVars().ErrInvalidArchive,
			arMagic + debianBinary[:62]:                                               GlobalVars().ErrInvalidArchive,
			arMagic + member("control.tar", "")[:58] + "\n\n":                         GlobalVars().ErrInvalidArchive,
			arMagic + fmt.Sprintf("%-48s%-10d`\n", "control.tar", -1):                 GlobalVars().ErrInvalidArchive,
			arMagic + member("dummy.txt", "x"):                                        GlobalVars().ErrMissingControlFile,
			arMagic + member("control.tar", "") + member("control.tar", buf.String()): GlobalVars().ErrMissingControlFile,
			arMagic + member("control.tar.foo", ""):                                   GlobalVars().ErrUnsupportedCompression,
		} {
			p, err := ParsePackage(strings.NewReader(data))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, expected, "%q", data)
		}
	})

	t.Run("Compression", func(t *testing.T) {
		cases := []struct {
			Extension     string
			WriterFactory func(io.Writer) io.WriteCloser
		}{
			{
				Extension: "",
				WriterFactory: func(w io.Writer) io.WriteCloser {
					return util.NopCloser{Writer: w}
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

				for _, suffix := range []string{"", "/"} {
					p, err := ParsePackage(strings.NewReader(arMagic + debianBinary + member("control.tar"+c.Extension+suffix, cbuf.String())))
					assert.NotNil(t, p)
					assert.NoError(t, err)
					assert.Equal(t, "gitea", p.Name)
				}
			})
		}
	})
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
			assert.ErrorIs(t, err, GlobalVars().ErrInvalidName)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		for _, version := range []string{"", "1-", ":1.0", "1_0"} {
			p, err := ParseControlFile(buildContent(packageName, version, packageArchitecture))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, GlobalVars().ErrInvalidVersion)
		}
	})

	t.Run("InvalidArchitecture", func(t *testing.T) {
		p, err := ParseControlFile(buildContent(packageName, packageVersion, ""))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, GlobalVars().ErrInvalidArchitecture)
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

	t.Run("ValidVersions", func(t *testing.T) {
		for _, version := range []string{"1.0", "0:1.2", "9:1.0", "10:1.0", "900:1a.2b-x-y_z~1+2"} {
			p, err := ParseControlFile(buildContent("testpkg", version, "amd64"))
			assert.NoError(t, err, "ParseControlFile with version %q", version)
			assert.NotNil(t, p)
		}
	})

	t.Run("SingleStanzaOnly", func(t *testing.T) {
		// A control file with a trailing stanza must not leak the extra fields into
		// p.Control, otherwise buildPackagesIndices would emit a second package entry
		// with an attacker-chosen Filename into the repository "Packages" index.
		content := bytes.NewBufferString("Package: realpkg\nVersion: 1.0.0\nArchitecture: amd64\nMaintainer: a <a@b.c>\nDescription: real\n\nPackage: openssl\nVersion: 99.0\nArchitecture: amd64\nFilename: pool/main/o/openssl/evil.deb\nDescription: spoofed\n")

		p, err := ParseControlFile(content)
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, "realpkg", p.Name)
		assert.Equal(t, "1.0.0", p.Version)
		assert.NotContains(t, p.Control, "openssl")
		assert.NotContains(t, p.Control, "evil.deb")
	})
}

func TestValidateDistributionOrComponent(t *testing.T) {
	bad := []string{
		"",
		".",
		"..",
		"-stable",
		".hidden",
		"a/b",
		"a b",
		"bookworm\nSigned-By: evil",
		"main\nFilename: pool/x",
		"a\tb",
	}
	for _, name := range bad {
		assert.False(t, IsValidDistributionOrComponent(name), "bad=%q", name)
	}

	good := []string{
		"stable",
		"bookworm",
		"bookworm-backports",
		"stable-updates",
		"main",
		"non-free-firmware",
		"a",
		"1",
	}
	for _, name := range good {
		assert.True(t, IsValidDistributionOrComponent(name), "good=%q", name)
	}
}

// TestParseControlFileMultilineDescription verifies a multi-line Description is assembled in order
// (the parser accumulates it in a strings.Builder); it guards the assembled value, not its timing.
func TestParseControlFileMultilineDescription(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("Package: testpkg\nVersion: 1.0\nArchitecture: amd64\nDescription: short summary\n more details\n even more\n")

	p, err := ParseControlFile(&buf)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "short summary more details even more", p.Metadata.Description)
}

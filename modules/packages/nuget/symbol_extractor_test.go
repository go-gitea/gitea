// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

const pdbContent = `QlNKQgEAAQAAAAAADAAAAFBEQiB2MS4wAAAAAAAABgB8AAAAWAAAACNQZGIAAAAA1AAAAAgBAAAj
fgAA3AEAAAQAAAAjU3RyaW5ncwAAAADgAQAABAAAACNVUwDkAQAAMAAAACNHVUlEAAAAFAIAACgB
AAAjQmxvYgAAAGm7ENm9SGxMtAFVvPUsPJTF6PbtAAAAAFcVogEJAAAAAQAAAA==`

func TestExtractPortablePdb(t *testing.T) {
	createArchive := func(name string, content []byte) []byte {
		var buf bytes.Buffer
		archive := zip.NewWriter(&buf)
		w, _ := archive.Create(name)
		w.Write(content)
		archive.Close()
		return buf.Bytes()
	}

	t.Run("MissingPdbFiles", func(t *testing.T) {
		var buf bytes.Buffer
		zip.NewWriter(&buf).Close()

		pdbs, err := ExtractPortablePdb(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		assert.ErrorIs(t, err, ErrMissingPdbFiles)
		assert.Empty(t, pdbs)
	})

	t.Run("InvalidFiles", func(t *testing.T) {
		data := createArchive("sub/test.bin", []byte{})

		pdbs, err := ExtractPortablePdb(bytes.NewReader(data), int64(len(data)))
		assert.ErrorIs(t, err, ErrInvalidFiles)
		assert.Empty(t, pdbs)
	})

	t.Run("Valid", func(t *testing.T) {
		b, _ := base64.StdEncoding.DecodeString(pdbContent)
		data := createArchive("test.pdb", b)

		pdbs, err := ExtractPortablePdb(bytes.NewReader(data), int64(len(data)))
		assert.NoError(t, err)
		assert.Len(t, pdbs, 1)
		assert.Equal(t, "test.pdb", pdbs[0].Name)
		assert.Equal(t, "d910bb6948bd4c6cb40155bcf52c3c94", pdbs[0].ID)
		pdbs.Close()
	})
}

func TestParseDebugHeaderID(t *testing.T) {
	t.Run("InvalidPdbMagicNumber", func(t *testing.T) {
		id, err := ParseDebugHeaderID(bytes.NewReader([]byte{0, 0, 0, 0}))
		assert.ErrorIs(t, err, ErrInvalidPdbMagicNumber)
		assert.Empty(t, id)
	})

	t.Run("MissingPdbStream", func(t *testing.T) {
		b, _ := base64.StdEncoding.DecodeString(`QlNKQgEAAQAAAAAADAAAAFBEQiB2MS4wAAAAAAAAAQB8AAAAWAAAACNVUwA=`)

		id, err := ParseDebugHeaderID(bytes.NewReader(b))
		assert.ErrorIs(t, err, ErrMissingPdbStream)
		assert.Empty(t, id)
	})

	t.Run("Valid", func(t *testing.T) {
		b, _ := base64.StdEncoding.DecodeString(pdbContent)

		id, err := ParseDebugHeaderID(bytes.NewReader(b))
		assert.NoError(t, err)
		assert.Equal(t, "d910bb6948bd4c6cb40155bcf52c3c94", id)
	})
}

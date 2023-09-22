// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	packageName    = "gitea.com/go-gitea/gitea"
	packageVersion = "v0.0.1"
)

func TestParsePackage(t *testing.T) {
	createArchive := func(files map[string][]byte) *bytes.Reader {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		for name, content := range files {
			w, _ := zw.Create(name)
			w.Write(content)
		}
		zw.Close()
		return bytes.NewReader(buf.Bytes())
	}

	t.Run("EmptyPackage", func(t *testing.T) {
		data := createArchive(nil)

		p, err := ParsePackage(data, int64(data.Len()))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidStructure)
	})

	t.Run("InvalidNameOrVersionStructure", func(t *testing.T) {
		data := createArchive(map[string][]byte{
			packageName + "/" + packageVersion + "/go.mod": {},
		})

		p, err := ParsePackage(data, int64(data.Len()))
		assert.Nil(t, p)
		assert.ErrorIs(t, err, ErrInvalidStructure)
	})

	t.Run("GoModFileInWrongDirectory", func(t *testing.T) {
		data := createArchive(map[string][]byte{
			packageName + "@" + packageVersion + "/subdir/go.mod": {},
		})

		p, err := ParsePackage(data, int64(data.Len()))
		assert.NotNil(t, p)
		assert.NoError(t, err)
		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, "module gitea.com/go-gitea/gitea", p.GoMod)
	})

	t.Run("Valid", func(t *testing.T) {
		data := createArchive(map[string][]byte{
			packageName + "@" + packageVersion + "/subdir/go.mod": []byte("invalid"),
			packageName + "@" + packageVersion + "/go.mod":        []byte("valid"),
		})

		p, err := ParsePackage(data, int64(data.Len()))
		assert.NotNil(t, p)
		assert.NoError(t, err)
		assert.Equal(t, packageName, p.Name)
		assert.Equal(t, packageVersion, p.Version)
		assert.Equal(t, "valid", p.GoMod)
	})
}

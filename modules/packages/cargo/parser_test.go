// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cargo

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	description = "Package Description"
	author      = "KN4CK3R"
	homepage    = "https://gitea.io/"
	license     = "MIT"
)

func TestParsePackage(t *testing.T) {
	createPackage := func(name, version string) io.Reader {
		metadata := `{
   "name":"` + name + `",
   "vers":"` + version + `",
   "description":"` + description + `",
   "authors": ["` + author + `"],
   "deps":[
      {
         "name":"dep",
         "version_req":"1.0"
      }
   ],
   "homepage":"` + homepage + `",
   "license":"` + license + `"
}`

		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, uint32(len(metadata)))
		buf.WriteString(metadata)
		binary.Write(&buf, binary.LittleEndian, uint32(4))
		buf.WriteString("test")
		return &buf
	}

	t.Run("InvalidName", func(t *testing.T) {
		for _, name := range []string{"", "0test", "-test", "_test", strings.Repeat("a", 65)} {
			data := createPackage(name, "1.0.0")

			cp, err := ParsePackage(data)
			assert.Nil(t, cp)
			assert.ErrorIs(t, err, ErrInvalidName)
		}
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		for _, version := range []string{"", "1.", "-1.0", "1.0.0/1"} {
			data := createPackage("test", version)

			cp, err := ParsePackage(data)
			assert.Nil(t, cp)
			assert.ErrorIs(t, err, ErrInvalidVersion)
		}
	})

	t.Run("Valid", func(t *testing.T) {
		data := createPackage("test", "1.0.0")

		cp, err := ParsePackage(data)
		assert.NotNil(t, cp)
		assert.NoError(t, err)

		assert.Equal(t, "test", cp.Name)
		assert.Equal(t, "1.0.0", cp.Version)
		assert.Equal(t, description, cp.Metadata.Description)
		assert.Equal(t, []string{author}, cp.Metadata.Authors)
		assert.Len(t, cp.Metadata.Dependencies, 1)
		assert.Equal(t, "dep", cp.Metadata.Dependencies[0].Name)
		assert.Equal(t, homepage, cp.Metadata.ProjectURL)
		assert.Equal(t, license, cp.Metadata.License)
		content, _ := io.ReadAll(cp.Content)
		assert.Equal(t, "test", string(content))
	})
}

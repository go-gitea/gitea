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

	for _, testCase := range []struct {
		name          string
		files         map[string][]byte
		expectedError error
		goMod         string
	}{
		{"EmptyPackage", nil, ErrInvalidStructure, ""},
		{"InvalidNameOrVersionStructure", map[string][]byte{
			packageName + "/" + packageVersion + "/go.mod": {},
		}, ErrInvalidStructure, ""},
		{"GoModFileInWrongDirectory", map[string][]byte{
			packageName + "@" + packageVersion + "/subdir/go.mod": {},
		}, nil, "module gitea.com/go-gitea/gitea"},
		{"InvalidVersion", map[string][]byte{
			packageName + "@v1.0.0\nv99.0.0/go.mod": []byte("module " + packageName),
		}, ErrInvalidVersion, ""},
		{"Valid", map[string][]byte{
			packageName + "@" + packageVersion + "/subdir/go.mod": []byte("invalid"),
			packageName + "@" + packageVersion + "/go.mod":        []byte("valid"),
		}, nil, "valid"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			data := createArchive(testCase.files)

			p, err := ParsePackage(data, int64(data.Len()))
			if testCase.expectedError != nil {
				assert.Nil(t, p)
				assert.ErrorIs(t, err, testCase.expectedError)
			} else {
				assert.NotNil(t, p)
				assert.NoError(t, err)
				assert.Equal(t, packageName, p.Name)
				assert.Equal(t, packageVersion, p.Version)
				assert.Equal(t, testCase.goMod, p.GoMod)
			}
		})
	}
}

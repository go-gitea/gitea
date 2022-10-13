// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rubygems

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePackageMetaData(t *testing.T) {
	createArchive := func(filename string, content []byte) io.Reader {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		hdr := &tar.Header{
			Name: filename,
			Mode: 0o600,
			Size: int64(len(content)),
		}
		tw.WriteHeader(hdr)
		tw.Write(content)
		tw.Close()
		return &buf
	}

	t.Run("MissingMetadataFile", func(t *testing.T) {
		data := createArchive("dummy.txt", []byte{0})

		rp, err := ParsePackageMetaData(data)
		assert.ErrorIs(t, err, ErrMissingMetadataFile)
		assert.Nil(t, rp)
	})

	t.Run("Valid", func(t *testing.T) {
		content, _ := base64.StdEncoding.DecodeString("H4sICHC/I2EEAG1ldGFkYXRhAAEeAOH/bmFtZTogZwp2ZXJzaW9uOgogIHZlcnNpb246IDEKWw35Tx4AAAA=")
		data := createArchive("metadata.gz", content)

		rp, err := ParsePackageMetaData(data)
		assert.NoError(t, err)
		assert.NotNil(t, rp)
	})
}

func TestParseMetadataFile(t *testing.T) {
	content, _ := base64.StdEncoding.DecodeString(`H4sIAMe7I2ECA9VVTW/UMBC9+1eYXvaUbJpSQBZUHJAqDlwK4kCFIseZzZrGH9iTqisEv52Js9nd
0KqggiqRXWnX45n3ZuZ5nCzL+JPQ15ulq7+AQnEORoj3HpReaSVRO8usNCB4qxEku4YQySbuCPo4
bjHOd07HeZGfMt9JXLlgBB9imOxx7UIULOPnCZMMLsDXXgeiYbW2jQ6C0y9TELBSa6kJ6/IzaySS
R1mUx1nxIitPeFGI9M2L6eGfWAMebANWaUgktzN9M3lsKNmxutBb1AYyCibbNhsDFu+q9GK/Tc4z
d2IcLBl9js5eHaXFsLyvXeNz0LQyL/YoLx8EsiCMBZlx46k6sS2PDD5AgA5kJPNKdhH2elWzOv7n
uv9Q9Aau/6ngP84elvNpXh5oRVlB5/yW7BH0+qu0G4gqaI/JdEHBFBS5l+pKtsARIjIwUnfj8Le0
+TrdJLl2DG5A9SjrjgZ1mG+4QbAD+G4ZZBUap6qVnnzGf6Rwp+vliBRqtnYGPBEKvkb0USyXE8mS
dVoR6hj07u0HZgAl3SRS8G/fmXcRK20jyq6rDMSYQFgidamqkXbbuspLXE/0k7GphtKqe67GuRC/
yjAbmt9LsOMp8xMamFkSQ38fP5EFjdz8LA4do2C69VvqWXAJgrPbKZb58/xZXrKoW6ttW13Bhvzi
4ftn7/yUxd4YGcglvTmmY8aGY3ZwRn4CqcWcidUGAAA=`)
	rp, err := parseMetadataFile(bytes.NewReader(content))
	assert.NoError(t, err)
	assert.NotNil(t, rp)

	assert.Equal(t, "gitea", rp.Name)
	assert.Equal(t, "1.0.5", rp.Version)
	assert.Equal(t, "ruby", rp.Metadata.Platform)
	assert.Equal(t, "Gitea package", rp.Metadata.Summary)
	assert.Equal(t, "RubyGems package test", rp.Metadata.Description)
	assert.Equal(t, []string{"Gitea"}, rp.Metadata.Authors)
	assert.Equal(t, "https://gitea.io/", rp.Metadata.ProjectURL)
	assert.Equal(t, []string{"MIT"}, rp.Metadata.Licenses)
	assert.Empty(t, rp.Metadata.RequiredRubygemsVersion)
	assert.Len(t, rp.Metadata.RequiredRubyVersion, 1)
	assert.Equal(t, ">=", rp.Metadata.RequiredRubyVersion[0].Restriction)
	assert.Equal(t, "2.3.0", rp.Metadata.RequiredRubyVersion[0].Version)
	assert.Len(t, rp.Metadata.RuntimeDependencies, 1)
	assert.Equal(t, "runtime-dep", rp.Metadata.RuntimeDependencies[0].Name)
	assert.Len(t, rp.Metadata.RuntimeDependencies[0].Version, 2)
	assert.Equal(t, ">=", rp.Metadata.RuntimeDependencies[0].Version[0].Restriction)
	assert.Equal(t, "1.2.0", rp.Metadata.RuntimeDependencies[0].Version[0].Version)
	assert.Equal(t, "<", rp.Metadata.RuntimeDependencies[0].Version[1].Restriction)
	assert.Equal(t, "2.0", rp.Metadata.RuntimeDependencies[0].Version[1].Version)
	assert.Len(t, rp.Metadata.DevelopmentDependencies, 1)
	assert.Equal(t, "dev-dep", rp.Metadata.DevelopmentDependencies[0].Name)
	assert.Len(t, rp.Metadata.DevelopmentDependencies[0].Version, 1)
	assert.Equal(t, "~>", rp.Metadata.DevelopmentDependencies[0].Version[0].Restriction)
	assert.Equal(t, "5.2", rp.Metadata.DevelopmentDependencies[0].Version[0].Version)
}

// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringContent(t *testing.T) {
	p := Pointer{Oid: "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393", Size: 1234}
	expected := "version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\n"
	assert.Equal(t, p.StringContent(), expected)
}

func TestRelativePath(t *testing.T) {
	p := Pointer{Oid: "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393"}
	expected := path.Join("4d", "7a", "214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393")
	assert.Equal(t, p.RelativePath(), expected)

	p2 := Pointer{Oid: "4d7a"}
	assert.Equal(t, p2.RelativePath(), "4d7a")
}

func TestIsValid(t *testing.T) {
	p := Pointer{}
	assert.False(t, p.IsValid())

	p = Pointer{Oid: "123"}
	assert.False(t, p.IsValid())

	p = Pointer{Oid: "z4cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc"}
	assert.False(t, p.IsValid())

	p = Pointer{Oid: "94cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc"}
	assert.True(t, p.IsValid())

	p = Pointer{Oid: "94cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc", Size: -1}
	assert.False(t, p.IsValid())
}

func TestGeneratePointer(t *testing.T) {
	p, err := GeneratePointer(strings.NewReader("Gitea"))
	assert.NoError(t, err)
	assert.True(t, p.IsValid())
	assert.Equal(t, p.Oid, "94cb57646c54a297c9807697e80a30946f79a4b82cb079d2606847825b1812cc")
	assert.Equal(t, p.Size, int64(5))
}

func TestReadPointerFromBuffer(t *testing.T) {
	p, err := ReadPointerFromBuffer([]byte{})
	assert.ErrorIs(t, err, ErrMissingPrefix)
	assert.False(t, p.IsValid())

	p, err = ReadPointerFromBuffer([]byte("test"))
	assert.ErrorIs(t, err, ErrMissingPrefix)
	assert.False(t, p.IsValid())

	p, err = ReadPointerFromBuffer([]byte("version https://git-lfs.github.com/spec/v1\n"))
	assert.ErrorIs(t, err, ErrInvalidStructure)
	assert.False(t, p.IsValid())

	p, err = ReadPointerFromBuffer([]byte("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a\nsize 1234\n"))
	assert.ErrorIs(t, err, ErrInvalidOIDFormat)
	assert.False(t, p.IsValid())

	p, err = ReadPointerFromBuffer([]byte("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a2146z4ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\n"))
	assert.ErrorIs(t, err, ErrInvalidOIDFormat)
	assert.False(t, p.IsValid())

	p, err = ReadPointerFromBuffer([]byte("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\ntest 1234\n"))
	assert.Error(t, err)
	assert.False(t, p.IsValid())

	p, err = ReadPointerFromBuffer([]byte("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize test\n"))
	assert.Error(t, err)
	assert.False(t, p.IsValid())

	p, err = ReadPointerFromBuffer([]byte("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\n"))
	assert.NoError(t, err)
	assert.True(t, p.IsValid())
	assert.Equal(t, p.Oid, "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393")
	assert.Equal(t, p.Size, int64(1234))

	p, err = ReadPointerFromBuffer([]byte("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\ntest"))
	assert.NoError(t, err)
	assert.True(t, p.IsValid())
	assert.Equal(t, p.Oid, "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393")
	assert.Equal(t, p.Size, int64(1234))
}

func TestReadPointer(t *testing.T) {
	p, err := ReadPointer(strings.NewReader("version https://git-lfs.github.com/spec/v1\noid sha256:4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393\nsize 1234\n"))
	assert.NoError(t, err)
	assert.True(t, p.IsValid())
	assert.Equal(t, p.Oid, "4d7a214614ab2935c943f9e0ff69d22eadbb8f32b1258daaa5e2ca24d17e2393")
	assert.Equal(t, p.Size, int64(1234))
}

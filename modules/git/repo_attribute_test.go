// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_nulSeparatedAttributeWriter_ReadAttribute(t *testing.T) {
	wr := &nulSeparatedAttributeWriter{
		attributes: make(chan attributeTriple, 5),
	}

	testStr := ".gitignore\"\n\x00linguist-vendored\x00unspecified\x00"

	n, err := wr.Write([]byte(testStr))

	assert.Len(t, testStr, n)
	assert.NoError(t, err)
	select {
	case attr := <-wr.ReadAttribute():
		assert.Equal(t, ".gitignore\"\n", attr.Filename)
		assert.Equal(t, "linguist-vendored", attr.Attribute)
		assert.Equal(t, "unspecified", attr.Value)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "took too long to read an attribute from the list")
	}
	// Write a second attribute again
	n, err = wr.Write([]byte(testStr))

	assert.Len(t, testStr, n)
	assert.NoError(t, err)

	select {
	case attr := <-wr.ReadAttribute():
		assert.Equal(t, ".gitignore\"\n", attr.Filename)
		assert.Equal(t, "linguist-vendored", attr.Attribute)
		assert.Equal(t, "unspecified", attr.Value)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "took too long to read an attribute from the list")
	}

	// Write a partial attribute
	_, err = wr.Write([]byte("incomplete-file"))
	assert.NoError(t, err)
	_, err = wr.Write([]byte("name\x00"))
	assert.NoError(t, err)

	select {
	case <-wr.ReadAttribute():
		assert.Fail(t, "There should not be an attribute ready to read")
	case <-time.After(100 * time.Millisecond):
	}
	_, err = wr.Write([]byte("attribute\x00"))
	assert.NoError(t, err)
	select {
	case <-wr.ReadAttribute():
		assert.Fail(t, "There should not be an attribute ready to read")
	case <-time.After(100 * time.Millisecond):
	}

	_, err = wr.Write([]byte("value\x00"))
	assert.NoError(t, err)

	attr := <-wr.ReadAttribute()
	assert.Equal(t, "incomplete-filename", attr.Filename)
	assert.Equal(t, "attribute", attr.Attribute)
	assert.Equal(t, "value", attr.Value)

	_, err = wr.Write([]byte("shouldbe.vendor\x00linguist-vendored\x00set\x00shouldbe.vendor\x00linguist-generated\x00unspecified\x00shouldbe.vendor\x00linguist-language\x00unspecified\x00"))
	assert.NoError(t, err)
	attr = <-wr.ReadAttribute()
	assert.NoError(t, err)
	assert.EqualValues(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: "linguist-vendored",
		Value:     "set",
	}, attr)
	attr = <-wr.ReadAttribute()
	assert.NoError(t, err)
	assert.EqualValues(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: "linguist-generated",
		Value:     "unspecified",
	}, attr)
	attr = <-wr.ReadAttribute()
	assert.NoError(t, err)
	assert.EqualValues(t, attributeTriple{
		Filename:  "shouldbe.vendor",
		Attribute: "linguist-language",
		Value:     "unspecified",
	}, attr)
}

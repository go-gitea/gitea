// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinioStorageIterator(t *testing.T) {
	config := MinioStorageConfig{
		Endpoint:        "127.0.0.1:9000",
		AccessKeyID:     "123456",
		SecretAccessKey: "12345678",
		Bucket:          "gitea",
		Location:        "us-east-1",
	}
	l, err := NewMinioStorage(context.Background(), config)
	assert.NoError(t, err)

	testFiles := [][]string{
		{"m/1.txt", "m1"},
		{"/mn/1.txt", "mn1"},
		{"b/1.txt", "b1"},
		{"b/2.txt", "b2"},
		{"b/3.txt", "b3"},
		{"b/x 4.txt", "bx4"},
	}
	for _, f := range testFiles {
		_, err = l.Save(f[0], bytes.NewBufferString(f[1]), -1)
		assert.NoError(t, err)
	}

	expectedList := map[string][]string{
		"mn":          {"mn/1.txt"},
		"m":           {"m/1.txt"},
		"b":           {"b/1.txt", "b/2.txt", "b/3.txt", "b/x 4.txt"},
		"m/b/../../m": {"m/1.txt"},
	}
	for dir, expected := range expectedList {
		count := 0
		err = l.IterateObjects(dir, func(path string, f Object) error {
			defer f.Close()
			assert.Contains(t, expected, path)
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Len(t, expected, count)
	}

}

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"testing"
)

func TestMinioStorageIterator(t *testing.T) {
	testStorageIterator(t, string(MinioStorageType), MinioStorageConfig{
		Endpoint:        "127.0.0.1:9000",
		AccessKeyID:     "123456",
		SecretAccessKey: "12345678",
		Bucket:          "gitea",
		Location:        "us-east-1",
	})
}

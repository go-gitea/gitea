// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"crypto/aes"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCopyFile(t *testing.T) {
	testContent := []byte("hello")

	tmpDir := t.TempDir()
	now := time.Now()
	srcFile := fmt.Sprintf("%s/copy-test-%d-src.txt", tmpDir, now.UnixMicro())
	dstFile := fmt.Sprintf("%s/copy-test-%d-dst.txt", tmpDir, now.UnixMicro())

	_ = os.Remove(srcFile)
	_ = os.Remove(dstFile)
	defer func() {
		_ = os.Remove(srcFile)
		_ = os.Remove(dstFile)
	}()

	err := os.WriteFile(srcFile, testContent, 0o777)
	assert.NoError(t, err)
	err = CopyFile(srcFile, dstFile)
	assert.NoError(t, err)
	dstContent, err := os.ReadFile(dstFile)
	assert.NoError(t, err)
	assert.Equal(t, testContent, dstContent)
}

func TestAESGCM(t *testing.T) {
	t.Parallel()

	key := make([]byte, aes.BlockSize)
	_, err := rand.Read(key)
	assert.NoError(t, err)

	plaintext := []byte("this will be encrypted")

	ciphertext, err := AESGCMEncrypt(key, plaintext)
	assert.NoError(t, err)

	decrypted, err := AESGCMDecrypt(key, ciphertext)
	assert.NoError(t, err)

	assert.Equal(t, plaintext, decrypted)
}

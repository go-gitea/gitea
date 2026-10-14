// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rpm

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"

	packages_module "gitea.dev/modules/packages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignPackageDeclaredHeaderSize(t *testing.T) {
	setting.AppDataPath = t.TempDir()

	priv, _, err := generateKeypair()
	require.NoError(t, err)

	// lead, empty signature header and a main header declaring 1 GiB of data
	var data bytes.Buffer
	data.Write([]byte{0xed, 0xab, 0xee, 0xdb})
	data.Write(make([]byte, 92))
	_ = binary.Write(&data, binary.BigEndian, []uint32{0x8eade801, 0, 0, 0, 0x8eade801, 0, 0, 1024 * 1024 * 1024})

	buf, err := packages_module.CreateHashedBufferFromReader(&data)
	require.NoError(t, err)
	defer buf.Close()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	signed, err := SignPackage(buf, priv)
	runtime.ReadMemStats(&after)
	assert.Nil(t, signed)
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(1024*1024))
}

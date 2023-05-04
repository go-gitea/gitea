// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dbfs

import (
	"bufio"
	"io"
	"os"
	"testing"

	"code.gitea.io/gitea/models/db"

	"github.com/stretchr/testify/assert"
)

func changeDefaultFileBlockSize(n int64) (restore func()) {
	old := defaultFileBlockSize
	defaultFileBlockSize = n
	return func() {
		defaultFileBlockSize = old
	}
}

func TestDbfsBasic(t *testing.T) {
	defer changeDefaultFileBlockSize(4)()

	// test basic write/read
	f, err := OpenFile(db.DefaultContext, "test.txt", os.O_RDWR|os.O_CREATE)
	assert.NoError(t, err)

	n, err := f.Write([]byte("0123456789")) // blocks: 0123 4567 89
	assert.NoError(t, err)
	assert.EqualValues(t, 10, n)

	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)

	buf, err := io.ReadAll(f)
	assert.NoError(t, err)
	assert.EqualValues(t, 10, n)
	assert.EqualValues(t, "0123456789", string(buf))

	// write some new data
	_, err = f.Seek(1, io.SeekStart)
	assert.NoError(t, err)
	_, err = f.Write([]byte("bcdefghi")) // blocks: 0bcd efgh i9
	assert.NoError(t, err)

	// read from offset
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.EqualValues(t, "9", string(buf))

	// read all
	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.EqualValues(t, "0bcdefghi9", string(buf))

	// write to new size
	_, err = f.Seek(-1, io.SeekEnd)
	assert.NoError(t, err)
	_, err = f.Write([]byte("JKLMNOP")) // blocks: 0bcd efgh iJKL MNOP
	assert.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.EqualValues(t, "0bcdefghiJKLMNOP", string(buf))

	// write beyond EOF and fill with zero
	_, err = f.Seek(5, io.SeekCurrent)
	assert.NoError(t, err)
	_, err = f.Write([]byte("xyzu")) // blocks: 0bcd efgh iJKL MNOP 0000 0xyz u
	assert.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.EqualValues(t, "0bcdefghiJKLMNOP\x00\x00\x00\x00\x00xyzu", string(buf))

	// write to the block with zeros
	_, err = f.Seek(-6, io.SeekCurrent)
	assert.NoError(t, err)
	_, err = f.Write([]byte("ABCD")) // blocks: 0bcd efgh iJKL MNOP 000A BCDz u
	assert.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.EqualValues(t, "0bcdefghiJKLMNOP\x00\x00\x00ABCDzu", string(buf))

	assert.NoError(t, f.Close())

	// test rename
	err = Rename(db.DefaultContext, "test.txt", "test2.txt")
	assert.NoError(t, err)

	_, err = OpenFile(db.DefaultContext, "test.txt", os.O_RDONLY)
	assert.Error(t, err)

	f, err = OpenFile(db.DefaultContext, "test2.txt", os.O_RDONLY)
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	// test remove
	err = Remove(db.DefaultContext, "test2.txt")
	assert.NoError(t, err)

	_, err = OpenFile(db.DefaultContext, "test2.txt", os.O_RDONLY)
	assert.Error(t, err)
}

func TestDbfsReadWrite(t *testing.T) {
	defer changeDefaultFileBlockSize(4)()

	f1, err := OpenFile(db.DefaultContext, "test.log", os.O_RDWR|os.O_CREATE)
	assert.NoError(t, err)
	defer f1.Close()

	f2, err := OpenFile(db.DefaultContext, "test.log", os.O_RDONLY)
	assert.NoError(t, err)
	defer f2.Close()

	_, err = f1.Write([]byte("line 1\n"))
	assert.NoError(t, err)

	f2r := bufio.NewReader(f2)

	line, err := f2r.ReadString('\n')
	assert.NoError(t, err)
	assert.EqualValues(t, "line 1\n", line)
	_, err = f2r.ReadString('\n')
	assert.ErrorIs(t, err, io.EOF)

	_, err = f1.Write([]byte("line 2\n"))
	assert.NoError(t, err)

	line, err = f2r.ReadString('\n')
	assert.NoError(t, err)
	assert.EqualValues(t, "line 2\n", line)
	_, err = f2r.ReadString('\n')
	assert.ErrorIs(t, err, io.EOF)
}

func TestDbfsSeekWrite(t *testing.T) {
	defer changeDefaultFileBlockSize(4)()

	f, err := OpenFile(db.DefaultContext, "test2.log", os.O_RDWR|os.O_CREATE)
	assert.NoError(t, err)
	defer f.Close()

	n, err := f.Write([]byte("111"))
	assert.NoError(t, err)

	_, err = f.Seek(int64(n), io.SeekStart)
	assert.NoError(t, err)

	_, err = f.Write([]byte("222"))
	assert.NoError(t, err)

	_, err = f.Seek(int64(n), io.SeekStart)
	assert.NoError(t, err)

	_, err = f.Write([]byte("333"))
	assert.NoError(t, err)

	fr, err := OpenFile(db.DefaultContext, "test2.log", os.O_RDONLY)
	assert.NoError(t, err)
	defer f.Close()

	buf, err := io.ReadAll(fr)
	assert.NoError(t, err)
	assert.EqualValues(t, "111333", string(buf))
}

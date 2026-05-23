// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dbfs

import (
	"bufio"
	"io"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDbfsBasic(t *testing.T) {
	defer test.MockVariableValue(&defaultFileBlockSize, 4)()

	// test basic write/read
	f, err := OpenFile(t.Context(), "test.txt", os.O_RDWR|os.O_CREATE)
	assert.NoError(t, err)

	n, err := f.Write([]byte("0123456789")) // blocks: 0123 4567 89
	assert.NoError(t, err)
	assert.Equal(t, 10, n)

	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)

	buf, err := io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, 10, n)
	assert.Equal(t, "0123456789", string(buf))

	// write some new data
	_, err = f.Seek(1, io.SeekStart)
	assert.NoError(t, err)
	_, err = f.Write([]byte("bcdefghi")) // blocks: 0bcd efgh i9
	assert.NoError(t, err)

	// read from offset
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, "9", string(buf))

	// read all
	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, "0bcdefghi9", string(buf))

	// write to new size
	_, err = f.Seek(-1, io.SeekEnd)
	assert.NoError(t, err)
	_, err = f.Write([]byte("JKLMNOP")) // blocks: 0bcd efgh iJKL MNOP
	assert.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, "0bcdefghiJKLMNOP", string(buf))

	// write beyond EOF and fill with zero
	_, err = f.Seek(5, io.SeekCurrent)
	assert.NoError(t, err)
	_, err = f.Write([]byte("xyzu")) // blocks: 0bcd efgh iJKL MNOP 0000 0xyz u
	assert.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, "0bcdefghiJKLMNOP\x00\x00\x00\x00\x00xyzu", string(buf))

	// write to the block with zeros
	_, err = f.Seek(-6, io.SeekCurrent)
	assert.NoError(t, err)
	_, err = f.Write([]byte("ABCD")) // blocks: 0bcd efgh iJKL MNOP 000A BCDz u
	assert.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	buf, err = io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, "0bcdefghiJKLMNOP\x00\x00\x00ABCDzu", string(buf))

	assert.NoError(t, f.Close())

	// test rename
	err = Rename(t.Context(), "test.txt", "test2.txt")
	assert.NoError(t, err)

	_, err = OpenFile(t.Context(), "test.txt", os.O_RDONLY)
	assert.Error(t, err)

	f, err = OpenFile(t.Context(), "test2.txt", os.O_RDONLY)
	assert.NoError(t, err)
	assert.NoError(t, f.Close())

	// test remove
	err = Remove(t.Context(), "test2.txt")
	assert.NoError(t, err)

	_, err = OpenFile(t.Context(), "test2.txt", os.O_RDONLY)
	assert.Error(t, err)

	// test stat
	f, err = OpenFile(t.Context(), "test/test.txt", os.O_RDWR|os.O_CREATE)
	assert.NoError(t, err)
	stat, err := f.Stat()
	assert.NoError(t, err)
	assert.Equal(t, "test.txt", stat.Name())
	assert.EqualValues(t, 0, stat.Size())
	_, err = f.Write([]byte("0123456789"))
	assert.NoError(t, err)
	stat, err = f.Stat()
	assert.NoError(t, err)
	assert.EqualValues(t, 10, stat.Size())

	t.Run("NonExisting", func(t *testing.T) {
		f, err := OpenFile(t.Context(), "non-existing.txt", os.O_RDONLY)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, f)

		f, err = OpenFile(t.Context(), "non-existing.txt", os.O_WRONLY)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, f)

		f, err = OpenFile(t.Context(), "non-existing.txt", os.O_WRONLY|os.O_APPEND|os.O_TRUNC)
		assert.ErrorIs(t, err, os.ErrNotExist)
		assert.Nil(t, f)
	})

	t.Run("Existing", func(t *testing.T) {
		assertFileContent := func(f File, expected string) {
			_, err := f.Seek(0, io.SeekStart)
			require.NoError(t, err)
			buf, err := io.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, expected, string(buf))
		}

		f, err := OpenFile(t.Context(), "existing.txt", os.O_RDWR|os.O_CREATE)
		require.NoError(t, err)
		_, _ = f.Write([]byte("test"))
		assertFileContent(f, "test")
		assert.NoError(t, f.Close())

		f, err = OpenFile(t.Context(), "existing.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND)
		require.NoError(t, err)
		_, _ = f.Write([]byte("\nnew"))
		assertFileContent(f, "test\nnew")
		assert.NoError(t, f.Close())

		f, err = OpenFile(t.Context(), "existing.txt", os.O_RDWR|os.O_TRUNC)
		require.NoError(t, err)
		assertFileContent(f, "")
		assert.NoError(t, f.Close())

		f, err = OpenFile(t.Context(), "existing.txt", os.O_RDWR|os.O_CREATE|os.O_EXCL)
		assert.ErrorIs(t, err, os.ErrExist)
		assert.Nil(t, f)
	})
}

func TestDbfsReadWrite(t *testing.T) {
	defer test.MockVariableValue(&defaultFileBlockSize, 4)()

	f1, err := OpenFile(t.Context(), "test.log", os.O_RDWR|os.O_CREATE)
	assert.NoError(t, err)
	defer f1.Close()

	f2, err := OpenFile(t.Context(), "test.log", os.O_RDONLY)
	assert.NoError(t, err)
	defer f2.Close()

	_, err = f1.Write([]byte("line 1\n"))
	assert.NoError(t, err)

	f2r := bufio.NewReader(f2)

	line, err := f2r.ReadString('\n')
	assert.NoError(t, err)
	assert.Equal(t, "line 1\n", line)
	_, err = f2r.ReadString('\n')
	assert.ErrorIs(t, err, io.EOF)

	_, err = f1.Write([]byte("line 2\n"))
	assert.NoError(t, err)

	line, err = f2r.ReadString('\n')
	assert.NoError(t, err)
	assert.Equal(t, "line 2\n", line)
	_, err = f2r.ReadString('\n')
	assert.ErrorIs(t, err, io.EOF)
}

func TestDbfsSeekWrite(t *testing.T) {
	defer test.MockVariableValue(&defaultFileBlockSize, 4)()

	// write something
	fw, err := OpenFile(t.Context(), "test2.log", os.O_RDWR|os.O_CREATE)
	require.NoError(t, err)
	defer fw.Close()

	n, err := fw.Write([]byte("111"))
	assert.NoError(t, err)

	_, err = fw.Seek(int64(n), io.SeekStart)
	assert.NoError(t, err)

	_, err = fw.Write([]byte("222"))
	assert.NoError(t, err)

	_, err = fw.Seek(int64(n), io.SeekStart)
	assert.NoError(t, err)

	_, err = fw.Write([]byte("333"))
	assert.NoError(t, err)

	// then read it
	fr, err := OpenFile(t.Context(), "test2.log", os.O_RDONLY)
	require.NoError(t, err)
	defer fr.Close()

	buf, err := io.ReadAll(fr)
	assert.NoError(t, err)
	assert.Equal(t, "111333", string(buf))
}

package nodb

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"os"

	"github.com/siddontang/go-snappy/snappy"
)

//dump format
// fileIndex(bigendian int64)|filePos(bigendian int64)
// |keylen(bigendian int32)|key|valuelen(bigendian int32)|value......
//
//key and value are both compressed for fast transfer dump on network using snappy

type BinLogAnchor struct {
	LogFileIndex int64
	LogPos       int64
}

func (m *BinLogAnchor) WriteTo(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, m.LogFileIndex); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, m.LogPos); err != nil {
		return err
	}
	return nil
}

func (m *BinLogAnchor) ReadFrom(r io.Reader) error {
	err := binary.Read(r, binary.BigEndian, &m.LogFileIndex)
	if err != nil {
		return err
	}

	err = binary.Read(r, binary.BigEndian, &m.LogPos)
	if err != nil {
		return err
	}

	return nil
}

func (l *Nodb) DumpFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return l.Dump(f)
}

func (l *Nodb) Dump(w io.Writer) error {
	m := new(BinLogAnchor)

	var err error

	l.wLock.Lock()
	defer l.wLock.Unlock()

	if l.binlog != nil {
		m.LogFileIndex = l.binlog.LogFileIndex()
		m.LogPos = l.binlog.LogFilePos()
	}

	wb := bufio.NewWriterSize(w, 4096)
	if err = m.WriteTo(wb); err != nil {
		return err
	}

	it := l.ldb.NewIterator()
	it.SeekToFirst()

	compressBuf := make([]byte, 4096)

	var key []byte
	var value []byte
	for ; it.Valid(); it.Next() {
		key = it.RawKey()
		value = it.RawValue()

		if key, err = snappy.Encode(compressBuf, key); err != nil {
			return err
		}

		if err = binary.Write(wb, binary.BigEndian, uint16(len(key))); err != nil {
			return err
		}

		if _, err = wb.Write(key); err != nil {
			return err
		}

		if value, err = snappy.Encode(compressBuf, value); err != nil {
			return err
		}

		if err = binary.Write(wb, binary.BigEndian, uint32(len(value))); err != nil {
			return err
		}

		if _, err = wb.Write(value); err != nil {
			return err
		}
	}

	if err = wb.Flush(); err != nil {
		return err
	}

	compressBuf = nil

	return nil
}

func (l *Nodb) LoadDumpFile(path string) (*BinLogAnchor, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return l.LoadDump(f)
}

func (l *Nodb) LoadDump(r io.Reader) (*BinLogAnchor, error) {
	l.wLock.Lock()
	defer l.wLock.Unlock()

	info := new(BinLogAnchor)

	rb := bufio.NewReaderSize(r, 4096)

	err := info.ReadFrom(rb)
	if err != nil {
		return nil, err
	}

	var keyLen uint16
	var valueLen uint32

	var keyBuf bytes.Buffer
	var valueBuf bytes.Buffer

	deKeyBuf := make([]byte, 4096)
	deValueBuf := make([]byte, 4096)

	var key, value []byte

	for {
		if err = binary.Read(rb, binary.BigEndian, &keyLen); err != nil && err != io.EOF {
			return nil, err
		} else if err == io.EOF {
			break
		}

		if _, err = io.CopyN(&keyBuf, rb, int64(keyLen)); err != nil {
			return nil, err
		}

		if key, err = snappy.Decode(deKeyBuf, keyBuf.Bytes()); err != nil {
			return nil, err
		}

		if err = binary.Read(rb, binary.BigEndian, &valueLen); err != nil {
			return nil, err
		}

		if _, err = io.CopyN(&valueBuf, rb, int64(valueLen)); err != nil {
			return nil, err
		}

		if value, err = snappy.Decode(deValueBuf, valueBuf.Bytes()); err != nil {
			return nil, err
		}

		if err = l.ldb.Put(key, value); err != nil {
			return nil, err
		}

		keyBuf.Reset()
		valueBuf.Reset()
	}

	deKeyBuf = nil
	deValueBuf = nil

	//if binlog enable, we will delete all binlogs and open a new one for handling simply
	if l.binlog != nil {
		l.binlog.PurgeAll()
	}

	return info, nil
}

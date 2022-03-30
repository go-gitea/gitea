// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package nuget

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/packages"
)

var (
	ErrMissingPdbFiles       = errors.New("Package does not contain PDB files")
	ErrInvalidFiles          = errors.New("Package contains invalid files")
	ErrInvalidPdbMagicNumber = errors.New("Invalid Portable PDB magic number")
	ErrMissingPdbStream      = errors.New("Missing PDB stream")
)

type PortablePdb struct {
	Name    string
	ID      string
	Content *packages.HashedBuffer
}

type PortablePdbList []*PortablePdb

func (l PortablePdbList) Close() {
	for _, pdb := range l {
		pdb.Content.Close()
	}
}

// ExtractPortablePdb extracts PDB files from a .snupkg file
func ExtractPortablePdb(r io.ReaderAt, size int64) (PortablePdbList, error) {
	archive, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	var pdbs PortablePdbList

	err = func() error {
		for _, file := range archive.File {
			if strings.HasSuffix(file.Name, "/") {
				continue
			}
			ext := strings.ToLower(filepath.Ext(file.Name))

			switch ext {
			case ".nuspec", ".xml", ".psmdcp", ".rels", ".p7s":
				continue
			case ".pdb":
				f, err := archive.Open(file.Name)
				if err != nil {
					return err
				}

				buf, err := packages.CreateHashedBufferFromReader(f, 32*1024*1024)

				f.Close()

				if err != nil {
					return err
				}

				id, err := ParseDebugHeaderID(buf)
				if err != nil {
					buf.Close()
					return fmt.Errorf("Invalid PDB file: %v", err)
				}

				if _, err := buf.Seek(0, io.SeekStart); err != nil {
					buf.Close()
					return err
				}

				pdbs = append(pdbs, &PortablePdb{
					Name:    path.Base(file.Name),
					ID:      id,
					Content: buf,
				})
			default:
				return ErrInvalidFiles
			}
		}
		return nil
	}()
	if err != nil {
		pdbs.Close()
		return nil, err
	}

	if len(pdbs) == 0 {
		return nil, ErrMissingPdbFiles
	}

	return pdbs, nil
}

// ParseDebugHeaderID TODO
func ParseDebugHeaderID(r io.ReadSeeker) (string, error) {
	var magic uint32
	if err := binary.Read(r, binary.LittleEndian, &magic); err != nil {
		return "", err
	}
	if magic != 0x424A5342 {
		return "", ErrInvalidPdbMagicNumber
	}

	if _, err := r.Seek(8, io.SeekCurrent); err != nil {
		return "", err
	}

	var versionStringSize int32
	if err := binary.Read(r, binary.LittleEndian, &versionStringSize); err != nil {
		return "", err
	}
	if _, err := r.Seek(int64(versionStringSize), io.SeekCurrent); err != nil {
		return "", err
	}
	if _, err := r.Seek(2, io.SeekCurrent); err != nil {
		return "", err
	}

	var streamCount int16
	if err := binary.Read(r, binary.LittleEndian, &streamCount); err != nil {
		return "", err
	}

	read4ByteAlignedString := func(r io.Reader) (string, error) {
		b := make([]byte, 4)
		var buf bytes.Buffer
		for {
			if _, err := r.Read(b); err != nil {
				return "", err
			}
			if i := bytes.IndexByte(b, 0); i != -1 {
				buf.Write(b[:i])
				return buf.String(), nil
			}
			buf.Write(b)
		}
	}

	for i := 0; i < int(streamCount); i++ {
		var offset uint32
		if err := binary.Read(r, binary.LittleEndian, &offset); err != nil {
			return "", err
		}
		if _, err := r.Seek(4, io.SeekCurrent); err != nil {
			return "", err
		}
		name, err := read4ByteAlignedString(r)
		if err != nil {
			return "", err
		}

		if name == "#Pdb" {
			if _, err := r.Seek(int64(offset), io.SeekStart); err != nil {
				return "", err
			}

			b := make([]byte, 16)
			if _, err := r.Read(b); err != nil {
				return "", err
			}

			data1 := binary.LittleEndian.Uint32(b[0:4])
			data2 := binary.LittleEndian.Uint16(b[4:6])
			data3 := binary.LittleEndian.Uint16(b[6:8])
			data4 := b[8:16]

			return fmt.Sprintf("%08x%04x%04x%04x%012x", data1, data2, data3, data4[:2], data4[2:]), nil
		}
	}

	return "", ErrMissingPdbStream
}

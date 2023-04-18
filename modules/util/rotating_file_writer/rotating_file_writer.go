// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rotating_file_writer

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/util"
)

type Options struct {
	Rotate           bool
	MaximumSize      int64
	RotateDaily      bool
	KeepDays         int
	Compress         bool
	CompressionLevel int
}

type RotatingFileWriter struct {
	mu sync.Mutex
	fd *os.File

	currentSize int64
	openDate    int

	options Options
}

func Open(filename string, options *Options) (*RotatingFileWriter, error) {
	if options == nil {
		options = &Options{}
	}

	rfw := &RotatingFileWriter{
		options: *options,
	}

	if err := rfw.open(filename); err != nil {
		return nil, err
	}

	return rfw, nil
}

func (rfw *RotatingFileWriter) Write(b []byte) (int, error) {
	if rfw.options.Rotate && ((rfw.options.MaximumSize > 0 && rfw.currentSize+int64(len(b)) >= rfw.options.MaximumSize) || (rfw.options.RotateDaily && time.Now().Day() != rfw.openDate)) {
		if err := rfw.DoRotate(); err != nil {
			return 0, err
		}
	}

	n, err := rfw.fd.Write(b)
	if err == nil {
		rfw.currentSize += int64(n)
	}
	return n, err
}

func (rfw *RotatingFileWriter) Flush() error {
	return rfw.fd.Sync()
}

func (rfw *RotatingFileWriter) Close() error {
	return rfw.fd.Close()
}

func (rfw *RotatingFileWriter) open(filename string) error {
	if rfw.fd != nil {
		if err := rfw.fd.Close(); err != nil {
			return err
		}
	}

	fd, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o660)
	if err != nil {
		return err
	}

	rfw.fd = fd

	finfo, err := fd.Stat()
	if err != nil {
		return err
	}
	rfw.currentSize = finfo.Size()
	rfw.openDate = time.Now().Day()

	return nil
}

func (rfw *RotatingFileWriter) ReleaseReopen() error {
	return rfw.open(rfw.fd.Name())
}

// Rotate the log file creating a backup like xx.2013-01-01.2
func (rfw *RotatingFileWriter) DoRotate() error {
	if !rfw.options.Rotate {
		return nil
	}

	rfw.mu.Lock()
	defer rfw.mu.Unlock()

	prefix := fmt.Sprintf("%s.%s.", rfw.fd.Name(), time.Now().Format("2006-01-02"))

	var err error
	fname := ""
	for i := 1; err == nil && i <= 999; i++ {
		fname = prefix + fmt.Sprintf("%03d", i)
		_, err = os.Lstat(fname)
		if rfw.options.Compress && err != nil {
			_, err = os.Lstat(fname + ".gz")
		}
	}
	// return error if the last file checked still existed
	if err == nil {
		return fmt.Errorf("cannot find free file to rename %s", rfw.fd.Name())
	}

	fd := rfw.fd
	if err := fd.Close(); err != nil { // close file before rename
		return err
	}

	rfw.fd = nil

	if err := util.Rename(fd.Name(), fname); err != nil {
		return err
	}

	if rfw.options.Compress {
		go compressOldFile(fname, rfw.options.CompressionLevel)
	}

	if err := rfw.open(fd.Name()); err != nil {
		return err
	}

	go deleteOldFiles(
		filepath.Dir(fd.Name()),
		filepath.Base(fd.Name()),
		time.Now().AddDate(0, 0, -rfw.options.KeepDays),
	)

	return nil
}

func compressOldFile(fname string, compressionLevel int) error {
	reader, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer reader.Close()

	buffer := bufio.NewReader(reader)
	fw, err := os.OpenFile(fname+".gz", os.O_WRONLY|os.O_CREATE, 0o660)
	if err != nil {
		return err
	}
	defer fw.Close()

	zw, err := gzip.NewWriterLevel(fw, compressionLevel)
	if err != nil {
		return err
	}
	defer zw.Close()

	_, err = buffer.WriteTo(zw)
	if err != nil {
		zw.Close()
		fw.Close()
		util.Remove(fname + ".gz")
		return err
	}
	reader.Close()

	return util.Remove(fname)
}

func deleteOldFiles(dir, prefix string, removeBefore time.Time) {
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				returnErr = fmt.Errorf("unable to delete old file '%s', error: %+v", path, r)
			}
		}()

		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(removeBefore) {
			if strings.HasPrefix(filepath.Base(path), prefix) {
				return util.Remove(path)
			}
		}
		return nil
	})
}

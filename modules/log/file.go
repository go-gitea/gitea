// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileLogger implements LoggerInterface.
// It writes messages by lines limit, file size limit, or time frequency.
type FileLogger struct {
	BaseLogger
	mw *MuxWriter
	// The opened file
	Filename string `json:"filename"`

	// Rotate at size
	Maxsize        int `json:"maxsize"`
	maxsizeCursize int

	// Rotate daily
	Daily         bool  `json:"daily"`
	Maxdays       int64 `json:"maxdays"`
	dailyOpenDate int

	Rotate bool `json:"rotate"`

	startLock sync.Mutex // Only one log can write to the file
}

// MuxWriter an *os.File writer with locker.
type MuxWriter struct {
	mu    *sync.Mutex
	fd    *os.File
	owner *FileLogger
}

// Write writes to os.File.
func (mw MuxWriter) Write(b []byte) (int, error) {
	if mw.mu == nil {
		mw.mu = &sync.Mutex{}
	}
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.owner.docheck(len(b))
	return mw.fd.Write(b)
}

// Close the internal writer
func (mw MuxWriter) Close() error {
	return mw.fd.Close()
}

// SetFd sets os.File in writer.
func (mw *MuxWriter) SetFd(fd *os.File) {
	if mw.fd != nil {
		mw.fd.Close()
	}
	mw.fd = fd
}

// NewFileLogger create a FileLogger returning as LoggerInterface.
func NewFileLogger() LoggerInterface {
	w := &FileLogger{
		Filename: "",
		Maxsize:  1 << 28, //256 MB
		Daily:    true,
		Maxdays:  7,
		Rotate:   true,
	}
	w.Level = TRACE
	// use MuxWriter instead direct use os.File for lock write when rotate
	w.mw = new(MuxWriter)
	w.mw.mu = &sync.Mutex{}
	w.mw.owner = w

	return w
}

// Init file logger with json config.
// config like:
//	{
//	"filename":"log/gogs.log",
//	"maxsize":1<<30,
//	"daily":true,
//	"maxdays":15,
//	"rotate":true
//	}
func (w *FileLogger) Init(config string) error {
	if err := json.Unmarshal([]byte(config), w); err != nil {
		return err
	}
	if len(w.Filename) == 0 {
		return errors.New("config must have filename")
	}
	// set MuxWriter as Logger's io.Writer
	w.createLogger(w.mw)
	return w.StartLogger()
}

// StartLogger start file logger. create log file and set to locker-inside file writer.
func (w *FileLogger) StartLogger() error {
	fd, err := w.createLogFile()
	if err != nil {
		return err
	}
	w.mw.SetFd(fd)
	return w.initFd()
}

func (w *FileLogger) docheck(size int) {
	w.startLock.Lock()
	defer w.startLock.Unlock()
	if w.Rotate && ((w.Maxsize > 0 && w.maxsizeCursize >= w.Maxsize) ||
		(w.Daily && time.Now().Day() != w.dailyOpenDate)) {
		if err := w.DoRotate(); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogger(%q): %s\n", w.Filename, err)
			return
		}
	}
	w.maxsizeCursize += size
}

func (w *FileLogger) createLogFile() (*os.File, error) {
	// Open the log file
	return os.OpenFile(w.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
}

func (w *FileLogger) initFd() error {
	fd := w.mw.fd
	finfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat: %v", err)
	}
	w.maxsizeCursize = int(finfo.Size())
	w.dailyOpenDate = time.Now().Day()
	return nil
}

// DoRotate means it need to write file in new file.
// new file name like xx.log.2013-01-01.2
func (w *FileLogger) DoRotate() error {
	_, err := os.Lstat(w.Filename)
	if err == nil { // file exists
		// Find the next available number
		num := 1
		fname := ""
		for ; err == nil && num <= 999; num++ {
			fname = w.Filename + fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), num)
			_, err = os.Lstat(fname)
		}
		// return error if the last file checked still existed
		if err == nil {
			return fmt.Errorf("rotate: cannot find free log number to rename %s", w.Filename)
		}

		fd := w.mw.fd
		fd.Close()

		// close fd before rename
		// Rename the file to its newfound home
		if err = os.Rename(w.Filename, fname); err != nil {
			return fmt.Errorf("Rotate: %v", err)
		}

		// re-start logger
		if err = w.StartLogger(); err != nil {
			return fmt.Errorf("Rotate StartLogger: %v", err)
		}

		go w.deleteOldLog()
	}

	return nil
}

func (w *FileLogger) deleteOldLog() {
	dir := filepath.Dir(w.Filename)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				returnErr = fmt.Errorf("Unable to delete old log '%s', error: %+v", path, r)
			}
		}()

		if !info.IsDir() && info.ModTime().Unix() < (time.Now().Unix()-60*60*24*w.Maxdays) {
			if strings.HasPrefix(filepath.Base(path), filepath.Base(w.Filename)) {

				if err := os.Remove(path); err != nil {
					returnErr = fmt.Errorf("Failed to remove %s: %v", path, err)
				}
			}
		}
		return returnErr
	})
}

// Flush flush file logger.
// there are no buffering messages in file logger in memory.
// flush file means sync file from disk.
func (w *FileLogger) Flush() {
	w.mw.fd.Sync()
}

func init() {
	Register("file", NewFileLogger)
}

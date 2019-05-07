package log

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var _ io.Writer = &Files{}

type ByType int

const (
	ByDay ByType = iota
	ByHour
	ByMonth
)

var (
	formats = map[ByType]string{
		ByDay:   "2006-01-02",
		ByHour:  "2006-01-02-15",
		ByMonth: "2006-01",
	}
)

func SetFileFormat(t ByType, format string) {
	formats[t] = format
}

func (b ByType) Format() string {
	return formats[b]
}

type Files struct {
	FileOptions
	f          *os.File
	lastFormat string
	lock       sync.Mutex
}

type FileOptions struct {
	Dir    string
	ByType ByType
	Loc    *time.Location
}

func prepareFileOption(opts []FileOptions) FileOptions {
	var opt FileOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.Dir == "" {
		opt.Dir = "./"
	}
	err := os.MkdirAll(opt.Dir, os.ModePerm)
	if err != nil {
		panic(err.Error())
	}

	if opt.Loc == nil {
		opt.Loc = time.Local
	}
	return opt
}

func NewFileWriter(opts ...FileOptions) *Files {
	opt := prepareFileOption(opts)
	return &Files{
		FileOptions: opt,
	}
}

func (f *Files) getFile() (*os.File, error) {
	var err error
	t := time.Now().In(f.Loc)
	if f.f == nil {
		f.lastFormat = t.Format(f.ByType.Format())
		f.f, err = os.OpenFile(filepath.Join(f.Dir, f.lastFormat+".log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		return f.f, err
	}
	if f.lastFormat != t.Format(f.ByType.Format()) {
		f.f.Close()
		f.lastFormat = t.Format(f.ByType.Format())
		f.f, err = os.OpenFile(filepath.Join(f.Dir, f.lastFormat+".log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		return f.f, err
	}
	return f.f, nil
}

func (f *Files) Write(bs []byte) (int, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	w, err := f.getFile()
	if err != nil {
		return 0, err
	}
	return w.Write(bs)
}

func (f *Files) Close() {
	if f.f != nil {
		f.f.Close()
		f.f = nil
	}
	f.lastFormat = ""
}

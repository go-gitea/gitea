package actions

import (
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
)

// ArtifactFile is a file that is stored in the artifact storage.
type ArtifactFile interface {
	fs.File
	io.Writer
}

type MkdirFS interface {
	MkdirAll(path string, perm fs.FileMode) error
	Open(name string) (ArtifactFile, error)
	OpenAtEnd(name string) (ArtifactFile, error)
}

var _ MkdirFS = (*DiskMkdirFs)(nil)

type DiskMkdirFs struct {
	dir string
}

func NewDiskMkdirFs(dir string) *DiskMkdirFs {
	return &DiskMkdirFs{dir: dir}
}

func (fsys DiskMkdirFs) MkdirAll(path string, perm fs.FileMode) error {
	fpath := filepath.Join(fsys.dir, path)
	return os.MkdirAll(fpath, perm)
}

func (fsys DiskMkdirFs) Open(name string) (ArtifactFile, error) {
	fpath := filepath.Join(fsys.dir, name)
	dirpath := filepath.Dir(fpath)
	os.MkdirAll(dirpath, os.ModePerm)
	return os.OpenFile(fpath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
}

func (fsys DiskMkdirFs) OpenAtEnd(name string) (ArtifactFile, error) {
	fpath := filepath.Join(fsys.dir, name)
	file, err := os.OpenFile(fpath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	_, err = file.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// https://stackoverflow.com/a/37382208
// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

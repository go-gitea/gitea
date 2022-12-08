package actions

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
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

func init() {
	db.RegisterModel(new(ActionArtifact))
}

// ActionArtifact is a file that is stored in the artifact storage.
type ActionArtifact struct {
	ID           int64
	JobID        int64
	RunnerID     int64  `xorm:"index"`
	RepoID       int64  `xorm:"index"`
	OwnerID      int64  `xorm:"index"`
	CommitSHA    string `xorm:"index"`
	FilePath     string
	FileSize     int64
	ArtifactPath string
	ArtifactName string
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated index"`
}

// CreateArtifact creates a new artifact with task info
func CreateArtifact(ctx context.Context, t *ActionTask) (*ActionArtifact, error) {
	artifact := &ActionArtifact{
		JobID:     t.JobID,
		RunnerID:  t.RunnerID,
		RepoID:    t.RepoID,
		OwnerID:   t.OwnerID,
		CommitSHA: t.CommitSHA,
	}
	_, err := db.GetEngine(ctx).Insert(artifact)
	return artifact, err
}

func GetArtifactByID(ctx context.Context, id int64) (*ActionArtifact, error) {
	var art ActionArtifact
	has, err := db.GetEngine(ctx).Where("id=?", id).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("task with id %d: %w", id, util.ErrNotExist)
	}

	return &art, nil
}

func UpdateArtifactByID(ctx context.Context, id int64, art *ActionArtifact) error {
	art.ID = id
	_, err := db.GetEngine(ctx).ID(id).AllCols().Update(art)
	return err
}

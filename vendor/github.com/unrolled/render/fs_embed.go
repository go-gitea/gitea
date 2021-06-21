// +build go1.16

package render

import (
	"embed"
	"io/fs"
	"path/filepath"
)

// EmbedFileSystem implements FileSystem on top of an embed.FS
type EmbedFileSystem struct {
	embed.FS
}

var _ FileSystem = &EmbedFileSystem{}

func (e *EmbedFileSystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return fs.WalkDir(e.FS, root, func(path string, d fs.DirEntry, _ error) error {
		if d == nil {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		return walkFn(path, info, err)
	})
}

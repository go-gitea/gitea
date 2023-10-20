// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package assetfs

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/util"

	"github.com/fsnotify/fsnotify"
)

// Layer represents a layer in a layered asset file-system. It has a name and works like http.FileSystem
type Layer struct {
	name      string
	fs        http.FileSystem
	localPath string
}

func (l *Layer) Name() string {
	return l.name
}

// Open opens the named file. The caller is responsible for closing the file.
func (l *Layer) Open(name string) (http.File, error) {
	return l.fs.Open(name)
}

// Local returns a new Layer with the given name, it serves files from the given local path.
func Local(name, base string, sub ...string) *Layer {
	// TODO: the old behavior (StaticRootPath might not be absolute), not ideal, just keep the same as before
	// Ideally, the caller should guarantee the base is absolute, guessing a relative path based on the current working directory is unreliable.
	base, err := filepath.Abs(base)
	if err != nil {
		// This should never happen in a real system. If it happens, the user must have already been in trouble: the system is not able to resolve its own paths.
		panic(fmt.Sprintf("Unable to get absolute path for %q: %v", base, err))
	}
	root := util.FilePathJoinAbs(base, sub...)
	return &Layer{name: name, fs: http.Dir(root), localPath: root}
}

// Bindata returns a new Layer with the given name, it serves files from the given bindata asset.
func Bindata(name string, fs http.FileSystem) *Layer {
	return &Layer{name: name, fs: fs}
}

// LayeredFS is a layered asset file-system. It works like http.FileSystem, but it can have multiple layers.
// The first layer is the top layer, and it will be used first.
// If the file is not found in the top layer, it will be searched in the next layer.
type LayeredFS struct {
	layers []*Layer
}

// Layered returns a new LayeredFS with the given layers. The first layer is the top layer.
func Layered(layers ...*Layer) *LayeredFS {
	return &LayeredFS{layers: layers}
}

// Open opens the named file. The caller is responsible for closing the file.
func (l *LayeredFS) Open(name string) (http.File, error) {
	for _, layer := range l.layers {
		f, err := layer.Open(name)
		if err == nil || !os.IsNotExist(err) {
			return f, err
		}
	}
	return nil, fs.ErrNotExist
}

// ReadFile reads the named file.
func (l *LayeredFS) ReadFile(elems ...string) ([]byte, error) {
	bs, _, err := l.ReadLayeredFile(elems...)
	return bs, err
}

// ReadLayeredFile reads the named file, and returns the layer name.
func (l *LayeredFS) ReadLayeredFile(elems ...string) ([]byte, string, error) {
	name := util.PathJoinRel(elems...)
	for _, layer := range l.layers {
		f, err := layer.Open(name)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, layer.name, err
		}
		bs, err := io.ReadAll(f)
		_ = f.Close()
		return bs, layer.name, err
	}
	return nil, "", fs.ErrNotExist
}

func shouldInclude(info fs.FileInfo, fileMode ...bool) bool {
	if util.CommonSkip(info.Name()) {
		return false
	}
	if len(fileMode) == 0 {
		return true
	} else if len(fileMode) == 1 {
		return fileMode[0] == !info.Mode().IsDir()
	}
	panic("too many arguments for fileMode in shouldInclude")
}

func readDir(layer *Layer, name string) ([]fs.FileInfo, error) {
	f, err := layer.Open(name)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdir(-1)
}

// ListFiles lists files/directories in the given directory. The fileMode controls the returned files.
// * omitted: all files and directories will be returned.
// * true: only files will be returned.
// * false: only directories will be returned.
// The returned files are sorted by name.
func (l *LayeredFS) ListFiles(name string, fileMode ...bool) ([]string, error) {
	fileSet := make(container.Set[string])
	for _, layer := range l.layers {
		infos, err := readDir(layer, name)
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			if shouldInclude(info, fileMode...) {
				fileSet.Add(info.Name())
			}
		}
	}
	files := fileSet.Values()
	sort.Strings(files)
	return files, nil
}

// ListAllFiles returns files/directories in the given directory, including subdirectories, recursively.
// The fileMode controls the returned files:
// * omitted: all files and directories will be returned.
// * true: only files will be returned.
// * false: only directories will be returned.
// The returned files are sorted by name.
func (l *LayeredFS) ListAllFiles(name string, fileMode ...bool) ([]string, error) {
	return listAllFiles(l.layers, name, fileMode...)
}

func listAllFiles(layers []*Layer, name string, fileMode ...bool) ([]string, error) {
	fileSet := make(container.Set[string])
	var list func(dir string) error
	list = func(dir string) error {
		for _, layer := range layers {
			infos, err := readDir(layer, dir)
			if err != nil {
				return err
			}
			for _, info := range infos {
				path := util.PathJoinRelX(dir, info.Name())
				if shouldInclude(info, fileMode...) {
					fileSet.Add(path)
				}
				if info.IsDir() {
					if err = list(path); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := list(name); err != nil {
		return nil, err
	}
	files := fileSet.Values()
	sort.Strings(files)
	return files, nil
}

// WatchLocalChanges watches local changes in the file-system. It's used to help to reload assets when the local file-system changes.
func (l *LayeredFS) WatchLocalChanges(ctx context.Context, callback func()) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Asset Local FileSystem Watcher", process.SystemProcessType, true)
	defer finished()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error("Unable to create watcher for asset local file-system: %v", err)
		return
	}
	defer watcher.Close()

	for _, layer := range l.layers {
		if layer.localPath == "" {
			continue
		}
		layerDirs, err := listAllFiles([]*Layer{layer}, ".", false)
		if err != nil {
			log.Error("Unable to list directories for asset local file-system %q: %v", layer.localPath, err)
			continue
		}
		layerDirs = append(layerDirs, ".")
		for _, dir := range layerDirs {
			if err = watcher.Add(util.FilePathJoinAbs(layer.localPath, dir)); err != nil && !os.IsNotExist(err) {
				log.Error("Unable to watch directory %s: %v", dir, err)
			}
		}
	}

	debounce := util.Debounce(100 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Trace("Watched asset local file-system had event: %v", event)
			debounce(callback)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Error("Watched asset local file-system had error: %v", err)
		}
	}
}

// GetFileLayerName returns the name of the first-seen layer that contains the given file.
func (l *LayeredFS) GetFileLayerName(elems ...string) string {
	name := util.PathJoinRel(elems...)
	for _, layer := range l.layers {
		f, err := layer.Open(name)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return ""
		}
		_ = f.Close()
		return layer.name
	}
	return ""
}

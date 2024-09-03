// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	GoogleDriveMimeFolder   string = "application/vnd.google-apps.folder"                // Folder mime type
	GoogleDriveMimeSyslink  string = "application/vnd.google-apps.shortcut"              // Syslink mime type
	GoogleListQueryWithName string = "trashed=false and '%s' in parents and name = '%s'" // Query files list with name
	GoogleListQuery         string = "trashed=false and '%s' in parents"                 // Query files list
)

type Gdrive struct {
	GoogleConfig *oauth2.Config // Google client app oauth project
	GoogleToken  *oauth2.Token  // Authenticated user
	driveService *drive.Service // Google drive service
	rootDrive    *drive.File    // Root to find files
}

func init() {
	RegisterStorageType(setting.GoogleDriveType, NewGoogleDrive)
}

// Create new Gdrive struct and configure google drive client
func NewGoogleDrive(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	gdrive := &Gdrive{
		GoogleConfig: &oauth2.Config{
			ClientID:     cfg.GoogleDriveConfig.Client,
			ClientSecret: cfg.GoogleDriveConfig.Secret,
			RedirectURL:  cfg.GoogleDriveConfig.Redirect,
			Scopes:       []string{drive.DriveScope, drive.DriveFileScope},
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.GoogleDriveConfig.AuthURI,
				TokenURL: cfg.GoogleDriveConfig.TokenURI,
			},
		},
		GoogleToken: &oauth2.Token{
			AccessToken:  cfg.GoogleDriveConfig.AccessToken,
			TokenType:    cfg.GoogleDriveConfig.TokenType,
			RefreshToken: cfg.GoogleDriveConfig.RefreshToken,
			Expiry:       cfg.GoogleDriveConfig.Expire,
		},
	}

	var err error
	if gdrive.driveService, err = drive.NewService(ctx, option.WithHTTPClient(gdrive.GoogleConfig.Client(ctx, gdrive.GoogleToken))); err != nil {
		return nil, err
	}

	if cfg.GoogleDriveConfig.RootFolder != "" {
		n := strings.Split(cfg.GoogleDriveConfig.RootFolder, "/")
		// Create folder with root id
		if strings.HasPrefix(n[0], "gdrive:") {
			if gdrive.rootDrive, err = gdrive.driveService.Files.Get(n[0][7:]).Fields("*").Do(); err != nil {
				return nil, fmt.Errorf("cannot get root: %v", err)
			}
			n = n[1:]
		} else if gdrive.rootDrive, err = gdrive.MkdirAll(strings.Join(n, "/")); err != nil {
			return nil, err
		}

		// resolve and create path not exists in new root
		if len(n) >= 1 {
			if gdrive.rootDrive, err = gdrive.MkdirAll(strings.Join(n, "/")); err != nil {
				return nil, err
			}
		}
	} else if gdrive.rootDrive, err = gdrive.driveService.Files.Get("root").Fields("*").Do(); err != nil {
		return nil, fmt.Errorf("cannot get root: %v", err)
	}

	log.Debug("gdrive: root folder name %q, id %q", gdrive.rootDrive.Name, gdrive.rootDrive.Id)
	return gdrive, nil
}

func (gdrive *Gdrive) cacheDelete(path string) error {
	if cc := cache.GetCache(); cc != nil {
		return cc.Delete(fmt.Sprintf("gdrive:%s:%s", gdrive.rootDrive.Id, gdrive.fixPath(path)))
	}
	return nil
}

func (gdrive *Gdrive) cacheGet(path string) *drive.File {
	if cc := cache.GetCache(); cc != nil {
		if str, ok := cc.Get(fmt.Sprintf("gdrive:%s:%s", gdrive.rootDrive.Id, gdrive.fixPath(path))); ok && str != "" {
			var node drive.File
			if err := json.Unmarshal([]byte(str), &node); err != nil {
				return nil
			} else if node.Id == "" {
				return nil
			}
			log.Debug("Gdrive cache get: %s", str)
			return &node
		}
	}
	return nil
}

func (gdrive *Gdrive) cachePut(path string, node *drive.File) error {
	if cc := cache.GetCache(); cc != nil {
		body, err := json.Marshal(node)
		if err != nil {
			return err
		}
		log.Debug("Gdrive cache put: %s", string(body))
		return cc.Put(fmt.Sprintf("gdrive:%s:%s", gdrive.rootDrive.Id, gdrive.fixPath(path)), string(body), 60*24*4)
	}
	return nil
}

func (*Gdrive) URL(path, name string) (*url.URL, error) {
	return nil, ErrURLNotSupported
}

// Get Node info and is not trashed/deleted
func (gdrive *Gdrive) resolveNode(folderID, name string) (*drive.File, error) {
	name = strings.ReplaceAll(strings.ReplaceAll(name, `\`, `\\`), `'`, `\'`)
	file, err := gdrive.driveService.Files.List().Fields("*").PageSize(300).Q(fmt.Sprintf(GoogleListQueryWithName, folderID, name)).Do()
	if err != nil {
		return nil, err
	} else if len(file.Files) != 1 {
		return nil, fs.ErrNotExist
	} else if len(file.Files) == 0 {
		return nil, fs.ErrNotExist
	} else if file.Files[0].Trashed {
		return file.Files[0], fs.ErrNotExist
	}
	return file.Files[0], nil
}

// List all files in folder
func (gdrive *Gdrive) listNodes(folderID string) ([]*drive.File, error) {
	var nodes []*drive.File
	folderGdrive := gdrive.driveService.Files.List().Fields("*").Q(fmt.Sprintf(GoogleListQuery, folderID)).PageSize(1000)
	for {
		res, err := folderGdrive.Do()
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, res.Files...)
		if res.NextPageToken == "" {
			break
		}
		folderGdrive.PageToken(res.NextPageToken)
	}
	return nodes, nil
}

// Split to nodes
func (*Gdrive) pathSplit(path string) []struct{ Name, Path string } {
	path = strings.TrimPrefix(strings.TrimSuffix(strings.ReplaceAll(strings.ReplaceAll(path, `\\`, "/"), `\`, "/"), "/"), "/")
	var nodes []struct{ Name, Path string }
	lastNode := 0
	for indexStr := range path {
		if path[indexStr] == '/' {
			nodes = append(nodes, struct{ Name, Path string }{path[lastNode:indexStr], path[0:indexStr]})
			lastNode = indexStr + 1
		}
	}
	nodes = append(nodes, struct{ Name, Path string }{path[lastNode:], path})
	return nodes
}

// Check if path have sub-folders
func (gdrive *Gdrive) checkMkdir(path string) bool { return len(gdrive.pathSplit(path)) > 1 }

// pretty path
func (gdrive *Gdrive) fixPath(path string) string { return gdrive.getLast(path).Path }

// pretty path and return last element
func (gdrive *Gdrive) getLast(path string) struct{ Name, Path string } {
	n := gdrive.pathSplit(path)
	return n[len(n)-1]
}

// Create recursive directory if not exists
func (gdrive *Gdrive) MkdirAll(path string) (*drive.File, error) {
	var current *drive.File
	if current = gdrive.cacheGet(gdrive.fixPath(path)); current != nil {
		return current, nil
	}

	current = gdrive.rootDrive      // root
	nodes := gdrive.pathSplit(path) // split node
	for nodeIndex, currentNode := range nodes {
		previus := current // storage previus Node
		if current = gdrive.cacheGet(currentNode.Path); current != nil {
			continue // continue to next node
		}

		var err error
		// Check if ared exist in folder
		if current, err = gdrive.resolveNode(previus.Id, currentNode.Name); err != nil {
			if err != fs.ErrNotExist {
				return nil, err // return drive error
			}

			// Base to create folder
			var folderCreate drive.File
			folderCreate.MimeType = GoogleDriveMimeFolder // folder mime
			folderCreate.Parents = []string{previus.Id}   // previus to folder to create

			// Create recursive folder
			for _, currentNode = range nodes[nodeIndex:] {
				folderCreate.Name = currentNode.Name // folder name
				if current, err = gdrive.driveService.Files.Create(&folderCreate).Fields("*").Do(); err != nil {
					return nil, err
				} else if err = gdrive.cachePut(currentNode.Path, current); err != nil {
					return nil, err
				}
				folderCreate.Parents[0] = current.Id // Set new root
			}

			// return new folder
			return current, nil
		} else if err = gdrive.cachePut(currentNode.Path, current); err != nil {
			return nil, err
		}
	}
	return current, nil
}

// Get *drive.File if exist
func (gdrive *Gdrive) GetNode(path string) (*drive.File, error) {
	var current *drive.File
	if current = gdrive.cacheGet(gdrive.fixPath(path)); current != nil {
		return current, nil
	}

	current = gdrive.rootDrive      // root
	nodes := gdrive.pathSplit(path) // split node
	for _, currentNode := range nodes {
		previus := current // storage previus Node
		if current = gdrive.cacheGet(currentNode.Path); current != nil {
			continue // continue to next node
		}

		var err error
		// Check if ared exist in folder
		if current, err = gdrive.resolveNode(previus.Id, currentNode.Name); err != nil {
			return nil, err // return drive error
		} else if err = gdrive.cachePut(currentNode.Path, current); err != nil {
			return nil, err
		}
	}
	return current, nil
}

type driveStat struct{ *drive.File }

func (node driveStat) IsDir() bool  { return node.File.MimeType == GoogleDriveMimeFolder }
func (node driveStat) Name() string { return node.File.Name }
func (node driveStat) Size() int64  { return node.File.Size }
func (node driveStat) Sys() any     { return nil }
func (node driveStat) ModTime() time.Time {
	err := fmt.Errorf("cannot get time")
	var t time.Time
	if node.File.ModifiedTime != "" {
		t, err = time.Parse(time.RFC3339, node.File.ModifiedTime)
	} else if node.File.CreatedTime != "" {
		t, err = time.Parse(time.RFC3339, node.File.CreatedTime)
	}
	if err != nil {
		panic(err)
	}
	return t
}

func (node driveStat) Mode() fs.FileMode {
	if node.File.MimeType == GoogleDriveMimeFolder {
		return fs.ModeDir | fs.ModePerm
	} else if node.File.MimeType == GoogleDriveMimeSyslink {
		return fs.ModeSymlink | fs.ModePerm
	}
	return fs.ModePerm
}

type driveOpen struct {
	node    *drive.File
	client  *drive.Service
	nodeRes *http.Response
	offset  int64
}

func (open *driveOpen) Stat() (fs.FileInfo, error) { return driveStat{open.node}, nil }
func (open *driveOpen) Close() error {
	if open.nodeRes == nil || open.nodeRes.Body == nil {
		return nil
	}
	err := open.nodeRes.Body.Close()
	open.nodeRes = nil
	open.offset = 0
	return err
}

func (open *driveOpen) Seek(offset int64, whence int) (int64, error) {
	log.Info("seeking %q: %d, %d", open.node.Id, offset, whence)
	if offset < 0 {
		return 0, errors.New("Seek: invalid offset")
	}

	switch whence {
	case io.SeekStart:
		if offset > open.node.Size {
			return 0, io.EOF
		}
		open.Close()
		node := open.client.Files.Get(open.node.Id).AcknowledgeAbuse(true)
		node.Header().Set("Range", fmt.Sprintf("bytes=%d-%d", offset, open.node.Size-1))
		var err error
		if open.nodeRes, err = node.Download(); err != nil {
			log.Error("gdrive download error: %s", err.Error())
			return 0, err
		}
		open.offset = offset
	case io.SeekCurrent:
		newOffset := open.offset + offset
		if newOffset < 0 || newOffset > open.node.Size {
			return 0, io.EOF
		}
		if _, err := io.CopyN(io.Discard, open, offset); err != nil {
			return 0, err
		}
		open.offset = newOffset
	case io.SeekEnd:
		newOffset := open.node.Size - offset
		if newOffset < 0 {
			return 0, io.EOF
		}
		open.Close()
		node := open.client.Files.Get(open.node.Id).AcknowledgeAbuse(true)
		node.Header().Set("Range", fmt.Sprintf("bytes=%d-%d", newOffset, open.node.Size-1))
		var err error
		if open.nodeRes, err = node.Download(); err != nil {
			log.Error("gdrive download error: %s", err.Error())
			return 0, err
		}
		open.offset = newOffset
	default:
		return 0, fs.ErrInvalid
	}

	return open.offset, nil
}

func (open *driveOpen) Read(p []byte) (int, error) {
	if open.nodeRes == nil || open.nodeRes.Body == nil {
		node := open.client.Files.Get(open.node.Id).AcknowledgeAbuse(true)
		if open.offset > 0 {
			node.Header().Set("Range", fmt.Sprintf("bytes=%d-%d", open.offset, open.node.Size-1))
		}
		var err error
		if open.nodeRes, err = node.Download(); err != nil {
			return 0, err
		}
	}

	n, err := open.nodeRes.Body.Read(p)
	if err != nil && err != io.EOF {
		return n, err
	}
	open.offset += int64(n)
	if err == io.EOF && open.offset >= open.node.Size {
		return n, io.EOF
	}
	return n, err
}

// resolve path and return File stream
func (gdrive *Gdrive) Open(path string) (Object, error) {
	fileNode, err := gdrive.GetNode(path)
	if err != nil {
		return nil, err
	}
	boot, err := gdrive.driveService.Files.Get(fileNode.Id).AcknowledgeAbuse(true).Download()
	if err != nil {
		return nil, err
	}
	return &driveOpen{fileNode, gdrive.driveService, boot, 0}, nil
}

func (gdrive *Gdrive) Stat(path string) (fs.FileInfo, error) {
	fileNode, err := gdrive.GetNode(path)
	if err != nil {
		return nil, err
	}
	return &driveStat{fileNode}, nil
}

func (gdrive *Gdrive) Delete(path string) error {
	fileNode, err := gdrive.GetNode(path)
	if err != nil {
		return err
	} else if err = gdrive.cacheDelete(path); err != nil {
		return err
	}
	return gdrive.driveService.Files.Delete(fileNode.Id).Do()
}

func (gdrive *Gdrive) IterateObjects(path string, iterator func(path string, obj Object) error) (err error) {
	var current *drive.File
	if current, err = gdrive.GetNode(path); err != nil {
		return err
	} else if current.MimeType != GoogleDriveMimeFolder {
		return iterator(gdrive.fixPath(path), &driveOpen{current, gdrive.driveService, nil, 0})
	}
	var recursiveCall func(path, folderID string) error
	recursiveCall = func(path, folderID string) error {
		files, err := gdrive.listNodes(folderID)
		if err != nil {
			return err
		}
		for _, k := range files {
			newPath := gdrive.fixPath(strings.Join([]string{path, k.Name}, "/"))
			if err = gdrive.cachePut(newPath, k); err != nil {
				return err
			} else if k.MimeType == GoogleDriveMimeFolder {
				if err = recursiveCall(newPath, k.Id); err != nil {
					return err
				}
			} else if err := iterator(newPath, &driveOpen{k, gdrive.driveService, nil, 0}); err != nil {
				return err
			}
		}
		return nil
	}
	return recursiveCall(gdrive.fixPath(path), current.Id)
}

func (gdrive *Gdrive) Save(path string, r io.Reader, size int64) (int64, error) {
	n := gdrive.pathSplit(path)
	if stat, err := gdrive.Stat(path); err == nil {
		res, err := gdrive.driveService.Files.Update(stat.(*driveStat).File.Id, nil).Media(r).Do()
		if err != nil {
			return 0, err
		} else if err = gdrive.cachePut(n[len(n)-1].Path, res); err != nil {
			return 0, err
		}
		return res.Size, nil
	}

	rootSolver := gdrive.rootDrive
	if gdrive.checkMkdir(path) {
		var err error
		if rootSolver, err = gdrive.MkdirAll(n[len(n)-2].Path); err != nil {
			return 0, err
		}
	}

	var err error
	if rootSolver, err = gdrive.driveService.Files.Create(&drive.File{MimeType: "application/octet-stream", Name: n[len(n)-1].Name, Parents: []string{rootSolver.Id}}).Fields("*").Media(r).Do(); err != nil {
		return 0, err
	} else if err = gdrive.cachePut(n[len(n)-1].Path, rootSolver); err != nil {
		return 0, err
	}
	return rootSolver.Size, nil
}

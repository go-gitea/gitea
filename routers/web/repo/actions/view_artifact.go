// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/public"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/typesniffer"
	"gitea.dev/modules/util"
	actions_service "gitea.dev/services/actions"
	context_module "gitea.dev/services/context"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type ArtifactPreviewFile struct {
	Path     string
	Name     string
	Link     string // empty for directories
	Depth    int
	Selected bool
}

type ArtifactPreviewTemplateData struct {
	RunURL                string
	RunIndex              int64
	RunAttempt            int64
	ArtifactName          string
	DownloadURL           string
	PreviewContentURL     string
	RequestedPathMissing  bool
	PreviewTooLarge       bool
	PreviewFilesTruncated bool
	PreviewFiles          []ArtifactPreviewFile
}

const artifactPreviewMaxFiles = 2000

type artifactPreviewList struct {
	paths     []string
	truncated bool
}

var artifactPreviewV4ZipListCache = expirable.NewLRU[[2]int64, artifactPreviewList](128, nil, 10*time.Minute)

type readAtBySeeker struct {
	rs  io.ReadSeeker
	pos int64
}

func (r *readAtBySeeker) ReadAt(p []byte, off int64) (int, error) {
	if off != r.pos { // some storages refetch on every seek, sequential reads must not
		if _, err := r.rs.Seek(off, io.SeekStart); err != nil {
			return 0, err
		}
	}
	n, err := io.ReadFull(r.rs, p)
	r.pos = off + int64(n)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return n, io.EOF
	}
	return n, err
}

// RenderArtifactPreview renders the file browser, previewLink and rawLink are the bases for file paths
func RenderArtifactPreview(ctx *context_module.Context, data *ArtifactPreviewTemplateData, paths []string, requested, previewLink, rawLink string) {
	if i, found := slices.BinarySearch(paths, requested); requested != "" && !found {
		if data.PreviewFilesTruncated {
			paths = slices.Insert(slices.Clone(paths), i, requested) // a capped listing cannot prove the file is missing, clone the cached slice
		} else {
			data.RequestedPathMissing = !data.PreviewTooLarge
			requested = ""
		}
	}
	data.PreviewFiles = buildArtifactPreviewFiles(paths, requested, previewLink)
	if requested != "" {
		data.PreviewContentURL = rawLink + util.PathEscapeSegments(requested)
	}
	ctx.Data["Title"] = ctx.Tr("preview")
	ctx.Data["ArtifactPreviewData"] = data
	ctx.HTML(http.StatusOK, tplArtifactPreviewAction)
}

// loadUploadedArtifactsByID loads all rows of the artifact, legacy artifacts store one row per file
func loadUploadedArtifactsByID(ctx context.Context, artifactID int64) ([]*actions_model.ActionArtifact, error) {
	art, exist, err := db.GetByID[actions_model.ActionArtifact](ctx, artifactID)
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, util.ErrNotExist
	}
	artifacts, err := actions_model.GetArtifactsByRunAttemptAndName(ctx, art.RunID, art.RunAttemptID, art.ArtifactName)
	if err != nil {
		return nil, err
	}
	if len(artifacts) == 0 || slices.ContainsFunc(artifacts, func(art *actions_model.ActionArtifact) bool {
		return art.Status != actions_model.ArtifactStatusUploadConfirmed
	}) {
		return nil, util.ErrNotExist
	}
	return artifacts, nil
}

func normalizeArtifactPreviewPath(path string) string {
	path = util.PathJoinRelX(path)
	if path == "." {
		return ""
	}
	return path
}

func artifactPreviewFallbackPath(artifact *actions_model.ActionArtifact) string {
	return util.IfZero(normalizeArtifactPreviewPath(artifact.ArtifactPath), artifact.ArtifactName)
}

func buildArtifactPreviewFiles(paths []string, selectedPath, previewLink string) []ArtifactPreviewFile {
	previewFiles := make([]ArtifactPreviewFile, 0, len(paths))
	seenDirs := map[string]struct{}{}
	for _, filePath := range paths {
		parts := strings.Split(filePath, "/")
		for i := range len(parts) - 1 {
			dirPath := strings.Join(parts[:i+1], "/")
			if _, ok := seenDirs[dirPath]; ok {
				continue
			}
			seenDirs[dirPath] = struct{}{}
			previewFiles = append(previewFiles, ArtifactPreviewFile{Path: dirPath, Name: parts[i], Depth: i})
		}
		previewFiles = append(previewFiles, ArtifactPreviewFile{
			Path:     filePath,
			Name:     path.Base(filePath),
			Link:     previewLink + util.PathEscapeSegments(filePath),
			Depth:    len(parts) - 1,
			Selected: filePath == selectedPath,
		})
	}
	return previewFiles
}

func newArtifactPreviewList(paths []string) artifactPreviewList {
	slices.Sort(paths)
	paths = slices.Compact(paths)
	// clone so the cached list does not keep the full backing array alive
	return artifactPreviewList{paths: slices.Clone(paths[:min(len(paths), artifactPreviewMaxFiles)]), truncated: len(paths) > artifactPreviewMaxFiles}
}

func isArtifactPreviewSizeAllowed(size int64) bool {
	maxSize := setting.Actions.ArtifactPreviewMaxSize
	return maxSize < 0 || maxSize > 0 && size <= maxSize
}

func artifactsTotalSize(artifacts []*actions_model.ActionArtifact) (size int64) {
	for _, art := range artifacts {
		size += art.FileSize
	}
	return size
}

func openArtifactV4ZipReader(artifact *actions_model.ActionArtifact) (storage.Object, *zip.Reader, error) {
	obj, err := storage.ActionsArtifacts.Open(artifact.StoragePath)
	if err != nil {
		return nil, nil, err
	}
	reader, err := zip.NewReader(&readAtBySeeker{rs: obj}, artifact.FileSize)
	if err != nil {
		_ = obj.Close()
		return nil, nil, err
	}
	return obj, reader, nil
}

func artifactV4ZipFilePath(file *zip.File) (string, bool) {
	if file.Mode().IsDir() {
		return "", false
	}
	path := normalizeArtifactPreviewPath(file.Name)
	return path, path != ""
}

func listPreviewForV4Artifact(artifact *actions_model.ActionArtifact) (artifactPreviewList, error) {
	key := [2]int64{artifact.ID, int64(artifact.UpdatedUnix)}
	if list, ok := artifactPreviewV4ZipListCache.Get(key); ok {
		return list, nil
	}

	obj, reader, err := openArtifactV4ZipReader(artifact)
	if errors.Is(err, zip.ErrFormat) {
		list := artifactPreviewList{paths: []string{artifactPreviewFallbackPath(artifact)}}
		artifactPreviewV4ZipListCache.Add(key, list)
		return list, nil
	} else if err != nil {
		return artifactPreviewList{}, err
	}
	defer obj.Close()

	paths := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		if path, ok := artifactV4ZipFilePath(file); ok {
			paths = append(paths, path)
		}
	}
	list := newArtifactPreviewList(paths)
	artifactPreviewV4ZipListCache.Add(key, list)
	return list, nil
}

func listPreview(artifacts []*actions_model.ActionArtifact) (artifactPreviewList, error) {
	if len(artifacts) == 1 && actions_service.IsArtifactV4(artifacts[0]) {
		return listPreviewForV4Artifact(artifacts[0])
	}
	paths := make([]string, len(artifacts))
	for i, artifact := range artifacts {
		paths[i] = artifactPreviewFallbackPath(artifact)
	}
	return newArtifactPreviewList(paths), nil
}

func artifactPreviewContentType(filename string, st typesniffer.SniffedType) string {
	switch ext := strings.ToLower(path.Ext(filename)); ext {
	case ".css", ".htm", ".html", ".js", ".mjs":
		return public.DetectWellKnownMimeType(ext)
	}
	if st.IsText() {
		return "text/plain; charset=utf-8"
	}
	return st.GetMimeType()
}

// ServeArtifactPreviewContent serves a previewable file, size must be the exact content length
func ServeArtifactPreviewContent(ctx *context_module.Base, filePath string, reader io.Reader, size int64) {
	if size < 0 || size > setting.UI.MaxDisplayFileSize {
		ctx.HTTPError(http.StatusRequestEntityTooLarge, "file is too large to preview, please download the artifact instead")
		return
	}
	sniffBuf, err := util.ReadWithLimit(reader, int(min(size, typesniffer.SniffContentSize)))
	if err != nil {
		log.Error("artifact preview ReadWithLimit: %v", err)
		ctx.HTTPError(http.StatusInternalServerError)
		return
	}
	st := typesniffer.DetectContentType(sniffBuf)
	if !st.IsText() && !st.IsImage() && !st.IsPDF() {
		ctx.HTTPError(http.StatusUnsupportedMediaType, "artifact preview is not supported for this file type")
		return
	}

	contentType := artifactPreviewContentType(filePath, st)
	var helperScript []byte
	if strings.HasPrefix(contentType, "text/html") {
		helperScript = []byte(htmlutil.HTMLFormat(`<script crossorigin src="%s"></script>`, public.AssetURI("web_src/js/external-render-helper.ts")))
	}
	var doctype []byte // the helper must follow a leading doctype, otherwise the page renders in quirks mode
	if end := bytes.IndexByte(sniffBuf, '>'); helperScript != nil && end != -1 && bytes.HasPrefix(bytes.ToLower(bytes.TrimLeft(sniffBuf[:end], "\ufeff \t\r\n")), []byte("<!doctype")) {
		doctype, sniffBuf = sniffBuf[:end+1], sniffBuf[end+1:]
	}
	contentLength := size + int64(len(helperScript))
	httplib.ServeSetHeaders(ctx.Resp, httplib.ServeHeaderOptions{
		Filename:           filePath,
		ContentDisposition: httplib.ContentDispositionInline,
		ContentType:        contentType,
		ContentLength:      &contentLength,
	})
	if helperScript != nil {
		ctx.Resp.Header().Set("Content-Security-Policy", "sandbox allow-scripts")
	}
	ctx.Resp.Header().Set("Access-Control-Allow-Origin", "*") // module scripts and fetch() from the sandboxed opaque origin need CORS
	_, _ = io.Copy(ctx.Resp, io.MultiReader(bytes.NewReader(doctype), bytes.NewReader(helperScript), bytes.NewReader(sniffBuf), io.LimitReader(reader, size-int64(len(sniffBuf)))))
}

func ArtifactsPreviewView(ctx *context_module.Context) {
	if setting.Actions.ArtifactPreviewMaxSize == 0 {
		ctx.NotFound(nil)
		return
	}
	artifacts, err := loadUploadedArtifactsByID(ctx, ctx.PathParamInt64("artifact_id"))
	if err == nil && artifacts[0].RepoID != ctx.Repo.Repository.ID {
		err = util.ErrNotExist
	}
	if err != nil {
		ctx.ServerError("loadUploadedArtifactsByID", err)
		return
	}
	artifact := artifacts[0]
	run, err := actions_model.GetRunByRepoAndID(ctx, artifact.RepoID, artifact.RunID)
	if err != nil {
		ctx.ServerError("GetRunByRepoAndID", err)
		return
	}
	run.Repo = ctx.Repo.Repository

	data := &ArtifactPreviewTemplateData{
		RunURL:          run.Link(),
		RunIndex:        run.Index,
		ArtifactName:    artifact.ArtifactName,
		DownloadURL:     run.Link() + "/artifacts/" + url.PathEscape(artifact.ArtifactName),
		PreviewTooLarge: !isArtifactPreviewSizeAllowed(artifactsTotalSize(artifacts)),
	}
	if artifact.RunAttemptID > 0 {
		attempt, err := actions_model.GetRunAttemptByRepoAndID(ctx, artifact.RepoID, artifact.RunAttemptID)
		if err != nil {
			ctx.ServerError("GetRunAttemptByRepoAndID", err)
			return
		}
		data.RunAttempt = attempt.Attempt
		data.RunURL += "/attempts/" + strconv.FormatInt(attempt.Attempt, 10)
		data.DownloadURL += "?attempt=" + strconv.FormatInt(attempt.Attempt, 10)
	}

	var list artifactPreviewList
	if !data.PreviewTooLarge {
		if list, err = listPreview(artifacts); err != nil {
			ctx.ServerError("listPreview", err)
			return
		}
	}
	data.PreviewFilesTruncated = list.truncated
	previewLink := fmt.Sprintf("%s/actions/artifacts/%d/preview/", ctx.Repo.RepoLink, artifact.ID)
	RenderArtifactPreview(ctx, data, list.paths, normalizeArtifactPreviewPath(ctx.PathParam("*")), previewLink, artifactPreviewRawLink(artifact.ID))
}

func artifactPreviewSignature(artifactID, expires int64) string {
	return base64.RawURLEncoding.EncodeToString(actions_module.BuildSignature("artifact-preview", strconv.FormatInt(artifactID, 10), strconv.FormatInt(expires, 10)))
}

// artifactPreviewRawLink is signed because the sandboxed preview frame's requests carry no session cookie
func artifactPreviewRawLink(artifactID int64) string {
	expires := int64(timeutil.TimeStampNow().AddDuration(time.Hour))
	return fmt.Sprintf("%s/-/actions/artifacts/%d/%d/%s/", setting.AppSubURL, artifactID, expires, artifactPreviewSignature(artifactID, expires))
}

func ArtifactsPreviewRawView(resp http.ResponseWriter, req *http.Request) {
	ctx := context_module.NewBaseContext(resp, req)
	artifactID, expires := ctx.PathParamInt64("artifact_id"), ctx.PathParamInt64("expires")
	if !setting.Actions.Enabled || setting.Actions.ArtifactPreviewMaxSize == 0 || expires < int64(timeutil.TimeStampNow()) ||
		!hmac.Equal([]byte(ctx.PathParam("signature")), []byte(artifactPreviewSignature(artifactID, expires))) {
		ctx.HTTPError(http.StatusNotFound)
		return
	}
	if err := serveArtifactPreviewRaw(ctx, artifactID, normalizeArtifactPreviewPath(ctx.PathParam("*"))); err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.HTTPError(http.StatusNotFound)
		} else {
			log.Error("serveArtifactPreviewRaw: %v", err)
			ctx.HTTPError(http.StatusInternalServerError)
		}
	}
}

func serveArtifactPreviewRaw(ctx *context_module.Base, artifactID int64, filePath string) error {
	artifacts, err := loadUploadedArtifactsByID(ctx, artifactID)
	if err != nil {
		return err
	}
	// only the preview page shows that the content is generated, so top-level navigations go there
	if ctx.Req.Header.Get("Sec-Fetch-Dest") == "document" {
		repo, err := repo_model.GetRepositoryByID(ctx, artifacts[0].RepoID)
		if err != nil {
			return err
		}
		ctx.Redirect(fmt.Sprintf("%s/actions/artifacts/%d/preview/%s", repo.Link(), artifactID, util.PathEscapeSegments(filePath)))
		return nil
	}
	if !isArtifactPreviewSizeAllowed(artifactsTotalSize(artifacts)) {
		ctx.HTTPError(http.StatusRequestEntityTooLarge, "artifact is too large to preview, please download it instead")
		return nil
	}

	if len(artifacts) == 1 && actions_service.IsArtifactV4(artifacts[0]) {
		obj, reader, err := openArtifactV4ZipReader(artifacts[0])
		if err == nil {
			defer obj.Close()
			idx := slices.IndexFunc(reader.File, func(file *zip.File) bool {
				path, ok := artifactV4ZipFilePath(file)
				return ok && path == filePath
			})
			if idx == -1 {
				return util.ErrNotExist
			}
			zipFile := reader.File[idx]
			entryReader, err := zipFile.Open()
			if err != nil {
				return err
			}
			defer entryReader.Close()
			ServeArtifactPreviewContent(ctx, filePath, entryReader, int64(zipFile.UncompressedSize64))
			return nil
		} else if !errors.Is(err, zip.ErrFormat) {
			return err
		}
	}

	idx := slices.IndexFunc(artifacts, func(art *actions_model.ActionArtifact) bool { return artifactPreviewFallbackPath(art) == filePath })
	if idx == -1 {
		return util.ErrNotExist
	}
	artifact := artifacts[idx]
	obj, err := storage.ActionsArtifacts.Open(artifact.StoragePath)
	if err != nil {
		return err
	}
	defer obj.Close()
	var reader io.Reader = obj
	if artifact.ContentEncodingOrType == actions_model.ContentEncodingV3Gzip {
		gzipReader, err := gzip.NewReader(obj)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	ServeArtifactPreviewContent(ctx, filePath, reader, artifact.FileSize)
	return nil
}

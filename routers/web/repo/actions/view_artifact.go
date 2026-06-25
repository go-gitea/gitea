// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/typesniffer"
	"gitea.dev/modules/util"
	"gitea.dev/modules/util/filebuffer"
	actions_service "gitea.dev/services/actions"
	context_module "gitea.dev/services/context"
)

type ArtifactsViewItem struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	ExpiresUnix int64  `json:"expiresUnix"`
}

type ArtifactPreviewFile struct {
	Path     string
	Selected bool
}

const (
	artifactPreviewV4ZipListCacheTTL        = 10 * time.Minute
	artifactPreviewV4ZipListCacheMaxEntries = 128
)

type artifactPreviewV4ZipListCacheEntry struct {
	paths     []string
	expiresAt time.Time
}

var artifactPreviewV4ZipListCache = struct {
	mu      sync.Mutex
	entries map[string]artifactPreviewV4ZipListCacheEntry
	order   []string
}{
	entries: map[string]artifactPreviewV4ZipListCacheEntry{},
}

type readAtBySeeker struct {
	rs io.ReadSeeker
	mu sync.Mutex
}

func (r *readAtBySeeker) ReadAt(p []byte, off int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.rs.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	n, err := io.ReadFull(r.rs, p)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return n, io.EOF
	}
	return n, err
}

// resolveArtifactAttemptIDFromQuery resolves the run_attempt_id used to scope artifact lookups.
// If the `attempt` query parameter is present and valid, it returns the matching attempt's ID.
// Otherwise it falls back to run.LatestAttemptID, which is 0 only for legacy runs created before ActionRunAttempt existed.
func resolveArtifactAttemptIDFromQuery(ctx *context_module.Context, run *actions_model.ActionRun) (int64, error) {
	if ctx.FormString("attempt") == "" {
		return run.LatestAttemptID, nil
	}
	attemptNum := ctx.FormInt64("attempt")
	if attemptNum <= 0 {
		return 0, util.ErrNotExist
	}
	attempt, err := actions_model.GetRunAttemptByRunIDAndAttemptNum(ctx, run.ID, attemptNum)
	if err != nil {
		return 0, err
	}
	return attempt.ID, nil
}

func getCurrentRunAndUploadedArtifacts(ctx *context_module.Context, artifactName string) (*actions_model.ActionRun, []*actions_model.ActionArtifact, bool) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return nil, nil, false
	}

	resolvedAttemptID, err := resolveArtifactAttemptIDFromQuery(ctx, run)
	if err != nil {
		ctx.NotFoundOrServerError("resolveArtifactAttemptIDFromQuery", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return nil, nil, false
	}

	artifacts, err := actions_model.GetArtifactsByRunAttemptAndName(ctx, run.ID, resolvedAttemptID, artifactName)
	if err != nil {
		ctx.ServerError("GetArtifactsByRunAttemptAndName", err)
		return nil, nil, false
	}
	if len(artifacts) == 0 {
		ctx.HTTPError(http.StatusNotFound, "artifact not found")
		return nil, nil, false
	}

	for _, art := range artifacts {
		if art.Status != actions_model.ArtifactStatusUploadConfirmed {
			ctx.HTTPError(http.StatusNotFound, "artifact not found")
			return nil, nil, false
		}
	}

	run.Repo = ctx.Repo.Repository
	return run, artifacts, true
}

func normalizeArtifactPreviewPath(path string) string {
	path = util.PathJoinRelX(path)
	if path == "." {
		return ""
	}
	return path
}

// GetRequestedPreviewPath reads the requested artifact preview path from a
// request, accepting either the trailing `/preview/raw/*` path segment or a
// `?path=` query parameter, and normalizes it to a safe relative path.
func GetRequestedPreviewPath(ctx *context_module.Context) string {
	path := strings.TrimPrefix(ctx.PathParam("*"), "/")
	if path == "" {
		path = ctx.Req.URL.Query().Get("path")
	}
	return normalizeArtifactPreviewPath(path)
}

func artifactPreviewFallbackPath(artifact *actions_model.ActionArtifact) string {
	path := normalizeArtifactPreviewPath(artifact.ArtifactPath)
	if path != "" {
		return path
	}
	return artifact.ArtifactName
}

// ChoosePreviewPath resolves the preview path to render.
// An empty `requested` means no path was specified, so the first file is selected as a default.
// A non-empty `requested` that is not present in `paths` returns "" so callers can 404 instead of silently swapping to a different file.
func ChoosePreviewPath(paths []string, requested string) string {
	if len(paths) == 0 {
		return ""
	}
	if requested == "" {
		return paths[0]
	}
	if util.SliceContainsString(paths, requested) {
		return requested
	}
	return ""
}

func listPreviewPathsForLegacyArtifacts(artifacts []*actions_model.ActionArtifact) []string {
	paths := make([]string, 0, len(artifacts))
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		path := artifactPreviewFallbackPath(artifact)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func openArtifactV4ZipReader(artifact *actions_model.ActionArtifact) (storage.Object, *zip.Reader, error) {
	f, err := storage.ActionsArtifacts.Open(artifact.StoragePath)
	if err != nil {
		return nil, nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}

	reader, err := zip.NewReader(&readAtBySeeker{rs: f}, stat.Size())
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return f, reader, nil
}

func listArtifactV4ZipFiles(reader *zip.Reader) ([]string, map[string]*zip.File) {
	paths := make([]string, 0, len(reader.File))
	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		path := normalizeArtifactPreviewPath(file.Name)
		if path == "" {
			continue
		}
		if _, ok := files[path]; ok {
			continue
		}
		files[path] = file
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, files
}

func listPreviewPathsForV4Artifact(artifact *actions_model.ActionArtifact) ([]string, error) {
	if paths, ok := getArtifactPreviewV4ZipListFromCache(artifact); ok {
		return paths, nil
	}

	obj, reader, err := openArtifactV4ZipReader(artifact)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) {
			paths := []string{artifactPreviewFallbackPath(artifact)}
			setArtifactPreviewV4ZipListCache(artifact, paths)
			return paths, nil
		}
		return nil, err
	}
	defer obj.Close()

	paths, _ := listArtifactV4ZipFiles(reader)
	setArtifactPreviewV4ZipListCache(artifact, paths)
	return paths, nil
}

func artifactPreviewV4ZipListCacheKey(artifact *actions_model.ActionArtifact) string {
	return strconv.FormatInt(artifact.ID, 10) + ":" + strconv.FormatInt(int64(artifact.UpdatedUnix), 10)
}

func removeArtifactPreviewV4ZipListCacheOrderKey(order []string, key string) []string {
	for i, current := range order {
		if current != key {
			continue
		}
		return append(order[:i], order[i+1:]...)
	}
	return order
}

func getArtifactPreviewV4ZipListFromCache(artifact *actions_model.ActionArtifact) ([]string, bool) {
	key := artifactPreviewV4ZipListCacheKey(artifact)

	artifactPreviewV4ZipListCache.mu.Lock()
	entry, ok := artifactPreviewV4ZipListCache.entries[key]
	if !ok {
		artifactPreviewV4ZipListCache.mu.Unlock()
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(artifactPreviewV4ZipListCache.entries, key)
		artifactPreviewV4ZipListCache.order = removeArtifactPreviewV4ZipListCacheOrderKey(artifactPreviewV4ZipListCache.order, key)
		artifactPreviewV4ZipListCache.mu.Unlock()
		return nil, false
	}
	paths := append([]string(nil), entry.paths...)
	artifactPreviewV4ZipListCache.mu.Unlock()
	return paths, true
}

func setArtifactPreviewV4ZipListCache(artifact *actions_model.ActionArtifact, paths []string) {
	key := artifactPreviewV4ZipListCacheKey(artifact)

	artifactPreviewV4ZipListCache.mu.Lock()
	defer artifactPreviewV4ZipListCache.mu.Unlock()

	if _, ok := artifactPreviewV4ZipListCache.entries[key]; ok {
		artifactPreviewV4ZipListCache.order = removeArtifactPreviewV4ZipListCacheOrderKey(artifactPreviewV4ZipListCache.order, key)
	}
	artifactPreviewV4ZipListCache.order = append(artifactPreviewV4ZipListCache.order, key)
	artifactPreviewV4ZipListCache.entries[key] = artifactPreviewV4ZipListCacheEntry{
		paths:     append([]string(nil), paths...),
		expiresAt: time.Now().Add(artifactPreviewV4ZipListCacheTTL),
	}

	for len(artifactPreviewV4ZipListCache.entries) > artifactPreviewV4ZipListCacheMaxEntries && len(artifactPreviewV4ZipListCache.order) > 0 {
		oldestKey := artifactPreviewV4ZipListCache.order[0]
		artifactPreviewV4ZipListCache.order = artifactPreviewV4ZipListCache.order[1:]
		if _, ok := artifactPreviewV4ZipListCache.entries[oldestKey]; !ok {
			continue
		}
		delete(artifactPreviewV4ZipListCache.entries, oldestKey)
	}
}

func listPreviewPaths(artifacts []*actions_model.ActionArtifact) ([]string, error) {
	if len(artifacts) == 1 && actions_service.IsArtifactV4(artifacts[0]) {
		return listPreviewPathsForV4Artifact(artifacts[0])
	}
	return listPreviewPathsForLegacyArtifacts(artifacts), nil
}

func isPreviewableArtifactType(st typesniffer.SniffedType) bool {
	return st.IsText() || st.IsPDF()
}

// WritePreviewRawError writes a minimal self-contained HTML error page directly to the response,
// bypassing Gitea's template system so the full Gitea UI is never rendered inside the preview iframe.
func WritePreviewRawError(ctx *context_module.Context, status int, msg string) {
	ctx.Resp.Header().Set("Content-Type", "text/html; charset=utf-8")
	ctx.Resp.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
	ctx.Resp.WriteHeader(status)
	_, _ = fmt.Fprintf(ctx.Resp, `<!DOCTYPE html><html><head><meta charset="utf-8">
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;color:#888}p{font-size:1.1em}</style>
</head><body><p>%s</p></body></html>`, html.EscapeString(msg))
}

func previewArtifactByReader(ctx *context_module.Context, path string, reader io.Reader) {
	maxSize := setting.UI.MaxDisplayFileSize
	buf := filebuffer.New(int(maxSize), "")
	defer buf.Close()
	// Copy maxSize+1 bytes so we can detect truncation: if the reader still has
	// data after the limit, the file is too large to render in the preview.
	n, err := io.Copy(buf, io.LimitReader(reader, maxSize+1))
	if err != nil {
		log.Error("artifact preview io.Copy: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to read artifact")
		return
	}
	if n > maxSize {
		WritePreviewRawError(ctx, http.StatusRequestEntityTooLarge, "file is too large to preview, please download the artifact instead")
		return
	}
	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		log.Error("artifact preview Seek: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to read artifact")
		return
	}
	previewArtifactByReadSeeker(ctx, path, buf)
}

func previewArtifactByReadSeeker(ctx *context_module.Context, path string, reader io.ReadSeeker) {
	buf := make([]byte, typesniffer.SniffContentSize)
	n, err := util.ReadAtMost(reader, buf)
	if err != nil {
		log.Error("artifact preview ReadAtMost: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to read artifact")
		return
	}
	buf = buf[:n]

	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		log.Error("artifact preview Seek: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to read artifact")
		return
	}

	st := typesniffer.DetectContentType(buf)
	if !isPreviewableArtifactType(st) {
		WritePreviewRawError(ctx, http.StatusUnsupportedMediaType, "artifact preview is not supported for this file type")
		return
	}

	// CSP sandbox is applied by httplib.ServeSetHeaders, see HINT: PDF-RENDER-SANDBOX
	ctx.ServeContent(reader, context_module.ServeHeaderOptions{
		Filename:    path,
		ContentType: st.GetMimeType(),
	})
}

func ArtifactsPreviewView(ctx *context_module.Context) {
	artifactName := ctx.PathParam("artifact_name")

	run, artifacts, ok := getCurrentRunAndUploadedArtifacts(ctx, artifactName)
	if !ok {
		return
	}

	paths, err := listPreviewPaths(artifacts)
	if err != nil {
		ctx.ServerError("listPreviewPaths", err)
		return
	}
	requested := GetRequestedPreviewPath(ctx)
	selectedPath := ChoosePreviewPath(paths, requested)

	previewFiles := make([]ArtifactPreviewFile, 0, len(paths))
	for _, path := range paths {
		previewFiles = append(previewFiles, ArtifactPreviewFile{
			Path:     path,
			Selected: path == selectedPath,
		})
	}

	runURL := run.Link()
	artifactPath := url.PathEscape(artifactName)
	previewURL := runURL + "/artifacts/" + artifactPath + "/preview"
	downloadURL := runURL + "/artifacts/" + artifactPath
	attemptQuery, attemptAmpQuery := "", ""
	if attempt := ctx.FormString("attempt"); attempt != "" {
		attemptQuery = "?attempt=" + url.QueryEscape(attempt)
		attemptAmpQuery = "&attempt=" + url.QueryEscape(attempt)
	}

	ctx.Data["Title"] = ctx.Tr("preview")
	ctx.Data["PageIsActions"] = true
	ctx.Data["RunURL"] = runURL
	ctx.Data["ArtifactName"] = artifactName
	ctx.Data["PreviewURL"] = previewURL
	ctx.Data["PreviewRawURL"] = previewURL + "/raw"
	ctx.Data["DownloadURL"] = downloadURL + attemptQuery
	ctx.Data["AttemptQuery"] = attemptQuery
	ctx.Data["AttemptAmpQuery"] = attemptAmpQuery
	ctx.Data["SelectedPath"] = selectedPath
	ctx.Data["RequestedPathMissing"] = requested != "" && selectedPath == ""
	ctx.Data["PreviewFiles"] = previewFiles

	ctx.HTML(http.StatusOK, tplArtifactPreviewAction)
}

// serveArtifactV4PreviewRaw opens the v4 artifact zip once and serves a single file from it,
// avoiding the redundant parse that listPreviewPaths would do for raw fetches.
func serveArtifactV4PreviewRaw(ctx *context_module.Context, artifact *actions_model.ActionArtifact, requested string) {
	obj, reader, err := openArtifactV4ZipReader(artifact)
	if err != nil {
		if !errors.Is(err, zip.ErrFormat) {
			log.Error("artifact preview openArtifactV4ZipReader: %v", err)
			WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to open artifact")
			return
		}
		fallbackPath := artifactPreviewFallbackPath(artifact)
		selectedPath := ChoosePreviewPath([]string{fallbackPath}, requested)
		if selectedPath == "" {
			WritePreviewRawError(ctx, http.StatusNotFound, "artifact file not found")
			return
		}
		f, err := storage.ActionsArtifacts.Open(artifact.StoragePath)
		if err != nil {
			log.Error("artifact preview ActionsArtifacts.Open: %v", err)
			WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to open artifact")
			return
		}
		defer f.Close()
		previewArtifactByReadSeeker(ctx, selectedPath, f)
		return
	}
	defer obj.Close()

	paths, files := listArtifactV4ZipFiles(reader)
	selectedPath := ChoosePreviewPath(paths, requested)
	if selectedPath == "" {
		WritePreviewRawError(ctx, http.StatusNotFound, "artifact file not found")
		return
	}
	zf := files[selectedPath]
	r, err := zf.Open()
	if err != nil {
		log.Error("artifact preview zip.File.Open: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to open artifact file")
		return
	}
	defer r.Close()
	previewArtifactByReader(ctx, selectedPath, r)
}

func ArtifactsPreviewRawView(ctx *context_module.Context) {
	artifactName := ctx.PathParam("artifact_name")

	_, artifacts, ok := getCurrentRunAndUploadedArtifacts(ctx, artifactName)
	if !ok {
		return
	}
	requested := GetRequestedPreviewPath(ctx)

	if len(artifacts) == 1 && actions_service.IsArtifactV4(artifacts[0]) {
		serveArtifactV4PreviewRaw(ctx, artifacts[0], requested)
		return
	}

	paths := listPreviewPathsForLegacyArtifacts(artifacts)
	selectedPath := ChoosePreviewPath(paths, requested)
	if selectedPath == "" {
		WritePreviewRawError(ctx, http.StatusNotFound, "artifact file not found")
		return
	}

	legacyByPath := make(map[string]*actions_model.ActionArtifact, len(artifacts))
	for _, artifact := range artifacts {
		path := artifactPreviewFallbackPath(artifact)
		if _, ok := legacyByPath[path]; ok {
			continue
		}
		legacyByPath[path] = artifact
	}

	artifact, ok := legacyByPath[selectedPath]
	if !ok {
		WritePreviewRawError(ctx, http.StatusNotFound, "artifact file not found")
		return
	}

	f, err := storage.ActionsArtifacts.Open(artifact.StoragePath)
	if err != nil {
		log.Error("artifact preview ActionsArtifacts.Open: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to open artifact")
		return
	}
	defer f.Close()

	if artifact.ContentEncodingOrType == actions_model.ContentEncodingV3Gzip {
		r, err := gzip.NewReader(f)
		if err != nil {
			log.Error("artifact preview gzip.NewReader: %v", err)
			WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to read artifact")
			return
		}
		defer r.Close()

		previewArtifactByReader(ctx, selectedPath, r)
		return
	}

	previewArtifactByReadSeeker(ctx, selectedPath, f)
}

func ArtifactsDeleteView(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}
	resolvedAttemptID, err := resolveArtifactAttemptIDFromQuery(ctx, run)
	if err != nil {
		ctx.NotFoundOrServerError("resolveArtifactAttemptIDFromQuery", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}
	artifactName := ctx.PathParam("artifact_name")
	if err := actions_model.SetArtifactNeedDeleteByRunAttempt(ctx, run.ID, resolvedAttemptID, artifactName); err != nil {
		ctx.ServerError("SetArtifactNeedDeleteByRunAttempt", err)
		return
	}
	ctx.JSON(http.StatusOK, struct{}{})
}

func ArtifactsDownloadView(ctx *context_module.Context) {
	run := getCurrentRunByPathParam(ctx)
	if ctx.Written() {
		return
	}
	resolvedAttemptID, err := resolveArtifactAttemptIDFromQuery(ctx, run)
	if err != nil {
		ctx.NotFoundOrServerError("resolveArtifactAttemptIDFromQuery", func(err error) bool {
			return errors.Is(err, util.ErrNotExist)
		}, err)
		return
	}
	artifactName := ctx.PathParam("artifact_name")
	artifacts, err := actions_model.GetArtifactsByRunAttemptAndName(ctx, run.ID, resolvedAttemptID, artifactName)
	if err != nil {
		ctx.ServerError("GetArtifactsByRunAttemptAndName", err)
		return
	}
	if len(artifacts) == 0 {
		ctx.HTTPError(http.StatusNotFound, "artifact not found")
		return
	}

	for _, art := range artifacts {
		if art.Status != actions_model.ArtifactStatusUploadConfirmed {
			ctx.HTTPError(http.StatusNotFound, "artifact not found")
			return
		}
	}

	ctx.Resp.Header().Set("Content-Disposition", httplib.EncodeContentDispositionAttachment(artifactName+".zip"))
	if len(artifacts) == 1 && actions_service.IsArtifactV4(artifacts[0]) {
		err := actions_service.DownloadArtifactV4(ctx.Base, artifacts[0])
		if err != nil {
			ctx.ServerError("DownloadArtifactV4", err)
			return
		}
		return
	}

	// Artifacts using the v1-v3 backend are stored as multiple individual files per artifact on the backend
	// Those need to be zipped for download
	zipWriter := zip.NewWriter(ctx.Resp)
	defer zipWriter.Close()

	writeArtifactToZip := func(art *actions_model.ActionArtifact) error {
		f, err := storage.ActionsArtifacts.Open(art.StoragePath)
		if err != nil {
			return fmt.Errorf("ActionsArtifacts.Open: %w", err)
		}
		defer f.Close()

		var r io.ReadCloser = f
		if art.ContentEncodingOrType == actions_model.ContentEncodingV3Gzip {
			r, err = gzip.NewReader(f)
			if err != nil {
				return fmt.Errorf("gzip.NewReader: %w", err)
			}
		}
		defer r.Close()

		w, err := zipWriter.Create(art.ArtifactPath)
		if err != nil {
			return fmt.Errorf("zipWriter.Create: %w", err)
		}
		_, err = io.Copy(w, r)
		if err != nil {
			return fmt.Errorf("io.Copy: %w", err)
		}
		return nil
	}

	for _, art := range artifacts {
		err := writeArtifactToZip(art)
		if err != nil {
			ctx.ServerError("writeArtifactToZip", err)
			return
		}
	}
}

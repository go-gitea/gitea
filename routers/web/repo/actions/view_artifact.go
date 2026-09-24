// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	pathpkg "path"
	"slices"
	"strconv"
	"strings"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/public"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/typesniffer"
	"gitea.dev/modules/util"
	"gitea.dev/modules/util/filebuffer"
	actions_service "gitea.dev/services/actions"
	context_module "gitea.dev/services/context"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type ArtifactsViewItem struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	ExpiresUnix int64  `json:"expiresUnix"`
	Previewable bool   `json:"previewable"`
}

type ArtifactPreviewFile struct {
	Path     string
	Name     string
	Link     string
	Depth    int
	IsDir    bool
	Selected bool
}

// ArtifactPreviewTemplateData contains the artifact preview page state.
type ArtifactPreviewTemplateData struct {
	RunURL                string
	RunIndex              int64
	RunAttempt            int64
	ArtifactName          string
	PreviewURL            string
	PreviewContentURL     string
	DownloadURL           string
	SelectedPath          string
	PreviewIsPDF          bool
	ShowPreviewContent    bool
	RequestedPathMissing  bool
	PreviewTooLarge       bool
	PreviewFilesTruncated bool
	PreviewFiles          []ArtifactPreviewFile
}

// PrepareArtifactPreviewTemplateData prepares the shared preview page data for production and devtest routes.
func PrepareArtifactPreviewTemplateData(ctx *context_module.Context, runURL string, runIndex, runAttempt int64, artifactName, requestedPath, selectedPath string, previewPaths []string, previewTooLarge, previewFilesTruncated bool) {
	artifactPath := url.PathEscape(artifactName)
	previewURL := runURL + "/artifacts/" + artifactPath + "/preview"
	backToRunURL := runURL
	attemptQuery := ""
	previewAttemptQuery := ""
	if runAttempt > 0 {
		attempt := strconv.FormatInt(runAttempt, 10)
		backToRunURL += "/attempts/" + attempt
		attemptQuery = "?attempt=" + attempt
		previewAttemptQuery = "&attempt=" + attempt
	}
	if selectedPath != "" && !slices.Contains(previewPaths, selectedPath) {
		previewPaths = insertArtifactPreviewPath(previewPaths, selectedPath)
	}
	previewFiles := BuildArtifactPreviewFiles(previewPaths, selectedPath)
	for i := range previewFiles {
		if !previewFiles[i].IsDir {
			previewFiles[i].Link = previewURL + "?path=" + url.QueryEscape(previewFiles[i].Path) + previewAttemptQuery
		}
	}
	previewContentURL := ""
	if selectedPath != "" {
		previewContentURL = previewURL + "/raw/" + escapeArtifactPreviewPath(selectedPath) + attemptQuery
	}
	ctx.Data["Title"] = ctx.Tr("preview")
	ctx.Data["PageIsActions"] = true
	ctx.Data["ArtifactPreviewData"] = ArtifactPreviewTemplateData{
		RunURL: backToRunURL, RunIndex: runIndex, RunAttempt: runAttempt, ArtifactName: artifactName,
		PreviewURL: previewURL, PreviewContentURL: previewContentURL, DownloadURL: runURL + "/artifacts/" + artifactPath + attemptQuery,
		SelectedPath: selectedPath, PreviewIsPDF: strings.EqualFold(pathpkg.Ext(selectedPath), ".pdf"),
		ShowPreviewContent: requestedPath != "" && selectedPath != "", RequestedPathMissing: requestedPath != "" && selectedPath == "" && !previewTooLarge,
		PreviewTooLarge: previewTooLarge, PreviewFilesTruncated: previewFilesTruncated, PreviewFiles: previewFiles,
	}
}

const (
	artifactPreviewV4ZipListCacheTTL         = 10 * time.Minute
	artifactPreviewV4ZipListCacheMaxEntries  = 128
	artifactPreviewMaxFiles                  = 2000
	artifactPreviewHTMLContentSecurityPolicy = "sandbox allow-scripts"
)

// artifactPreviewList is one artifact's file listing for the preview browser.
// `paths` is sorted and capped at artifactPreviewMaxFiles; `truncated` reports whether the cap dropped entries.
type artifactPreviewList struct {
	paths     []string
	truncated bool
}

// artifactPreviewV4ZipListCache caches each v4 artifact's listing, keyed by artifact ID and update time.
// Cached listings are capped, so an artifact with a huge number of entries cannot pin unbounded memory here.
var artifactPreviewV4ZipListCache = expirable.NewLRU[string, artifactPreviewList](artifactPreviewV4ZipListCacheMaxEntries, nil, artifactPreviewV4ZipListCacheTTL)

type readAtBySeeker struct {
	rs io.ReadSeeker
}

func (r *readAtBySeeker) ReadAt(p []byte, off int64) (int, error) {
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
// If an `attempt` query parameter is present and valid, it returns the matching attempt's ID.
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

	// if artifacts status is not uploaded-confirmed, treat it as not found
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

func escapeArtifactPreviewPath(path string) string {
	segments := strings.Split(path, "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return strings.Join(segments, "/")
}

func artifactPreviewFallbackPath(artifact *actions_model.ActionArtifact) string {
	path := normalizeArtifactPreviewPath(artifact.ArtifactPath)
	if path != "" {
		return path
	}
	return artifact.ArtifactName
}

func ChoosePreviewPath(paths []string, requested string) string {
	if requested != "" && slices.Contains(paths, requested) {
		return requested
	}
	return ""
}

// BuildArtifactPreviewFiles builds a directory-grouped file tree for the artifact browser.
func BuildArtifactPreviewFiles(paths []string, selectedPath string) []ArtifactPreviewFile {
	previewFiles := make([]ArtifactPreviewFile, 0, len(paths))
	seenDirs := map[string]struct{}{}
	for _, filePath := range paths {
		parts := strings.Split(filePath, "/")
		for i := 0; i < len(parts)-1; i++ {
			dirPath := strings.Join(parts[:i+1], "/")
			if _, ok := seenDirs[dirPath]; ok {
				continue
			}
			seenDirs[dirPath] = struct{}{}
			previewFiles = append(previewFiles, ArtifactPreviewFile{
				Path:  dirPath,
				Name:  parts[i],
				Depth: i,
				IsDir: true,
			})
		}
		previewFiles = append(previewFiles, ArtifactPreviewFile{
			Path:     filePath,
			Name:     pathpkg.Base(filePath),
			Depth:    len(parts) - 1,
			Selected: filePath == selectedPath,
		})
	}
	return previewFiles
}

// capArtifactPreviewPaths caps a sorted path list at artifactPreviewMaxFiles.
// The cap bounds what the listing cache retains, so an artifact with a huge number of entries cannot pin memory.
func capArtifactPreviewPaths(paths []string) ([]string, bool) {
	if len(paths) <= artifactPreviewMaxFiles {
		return paths, false
	}
	// copy instead of reslicing, otherwise the cached slice keeps the full backing array alive
	return append([]string(nil), paths[:artifactPreviewMaxFiles]...), true
}

// insertArtifactPreviewPath inserts into a sorted copy: BuildArtifactPreviewFiles needs sorted input, and the caller's slice may be the shared cached one
func insertArtifactPreviewPath(paths []string, path string) []string {
	i, _ := slices.BinarySearch(paths, path)
	return slices.Insert(slices.Clone(paths), i, path)
}

func isArtifactPreviewSizeAllowed(size int64) bool {
	maxSize := setting.Actions.ArtifactPreviewMaxSize
	if maxSize < 0 {
		return true
	}
	return maxSize > 0 && size <= maxSize
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
	slices.Sort(paths)
	return paths
}

func listPreviewForLegacyArtifacts(artifacts []*actions_model.ActionArtifact) artifactPreviewList {
	paths, truncated := capArtifactPreviewPaths(listPreviewPathsForLegacyArtifacts(artifacts))
	return artifactPreviewList{paths: paths, truncated: truncated}
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

func listArtifactV4ZipPaths(reader *zip.Reader) artifactPreviewList {
	paths := make([]string, 0, len(reader.File))
	seen := make(map[string]struct{}, len(reader.File))
	for _, file := range reader.File {
		path, ok := artifactV4ZipFilePath(file)
		if !ok {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	slices.Sort(paths)
	capped, truncated := capArtifactPreviewPaths(paths)
	return artifactPreviewList{paths: capped, truncated: truncated}
}

func findArtifactV4ZipFile(reader *zip.Reader, requested string) *zip.File {
	for _, file := range reader.File {
		path, ok := artifactV4ZipFilePath(file)
		if !ok {
			continue
		}
		if path == requested {
			return file
		}
	}
	return nil
}

func artifactV4ZipFilePath(file *zip.File) (string, bool) {
	if file.FileInfo().IsDir() {
		return "", false
	}
	path := normalizeArtifactPreviewPath(file.Name)
	return path, path != ""
}

func listPreviewForV4Artifact(artifact *actions_model.ActionArtifact) (artifactPreviewList, error) {
	key := artifactPreviewV4ZipListCacheKey(artifact)
	if list, ok := artifactPreviewV4ZipListCache.Get(key); ok {
		return list, nil
	}

	obj, reader, err := openArtifactV4ZipReader(artifact)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) {
			list := artifactPreviewList{paths: []string{artifactPreviewFallbackPath(artifact)}}
			artifactPreviewV4ZipListCache.Add(key, list)
			return list, nil
		}
		return artifactPreviewList{}, err
	}
	defer obj.Close()

	list := listArtifactV4ZipPaths(reader)
	artifactPreviewV4ZipListCache.Add(key, list)
	return list, nil
}

func artifactPreviewV4ZipListCacheKey(artifact *actions_model.ActionArtifact) string {
	return strconv.FormatInt(artifact.ID, 10) + ":" + strconv.FormatInt(int64(artifact.UpdatedUnix), 10)
}

func listPreview(artifacts []*actions_model.ActionArtifact) (artifactPreviewList, error) {
	if len(artifacts) == 1 && actions_service.IsArtifactV4(artifacts[0]) {
		return listPreviewForV4Artifact(artifacts[0])
	}
	return listPreviewForLegacyArtifacts(artifacts), nil
}

func isPreviewableArtifactType(st typesniffer.SniffedType) bool {
	return st.IsText() || st.IsImage() || st.IsPDF()
}

func artifactPreviewContentType(filename string, st typesniffer.SniffedType) string {
	switch strings.ToLower(pathpkg.Ext(filename)) {
	case ".css", ".htm", ".html", ".js", ".mjs":
		if contentType := mime.TypeByExtension(pathpkg.Ext(filename)); contentType != "" {
			return contentType
		}
	}
	if st.IsText() {
		return "text/plain; charset=utf-8"
	}
	return st.GetMimeType()
}

func artifactPreviewHTMLReader(ctx *context_module.Context, reader io.Reader) (*filebuffer.FileBackedBuffer, error) {
	buf := filebuffer.New(int(setting.UI.MaxDisplayFileSize), "")
	_, err := fmt.Fprintf(buf, `<script crossorigin src=%q id="gitea-external-render-helper" data-render-query-string=%q></script>`, public.AssetURI("web_src/js/external-render-helper.ts"), html.EscapeString(ctx.Req.URL.RawQuery))
	if err == nil {
		_, err = io.Copy(buf, reader)
	}
	if err != nil {
		_ = buf.Close()
		return nil, err
	}
	return buf, nil
}

func artifactPreviewServeHeaderOptions(path string, st typesniffer.SniffedType) context_module.ServeHeaderOptions {
	contentType := artifactPreviewContentType(path, st)
	opts := context_module.ServeHeaderOptions{
		Filename:           path,
		ContentDisposition: httplib.ContentDispositionInline,
		ContentType:        contentType,
	}
	if strings.HasPrefix(contentType, "text/html") {
		opts.ContentSecurityPolicy = artifactPreviewHTMLContentSecurityPolicy
	}
	return opts
}

func WritePreviewRawError(ctx *context_module.Context, status int, msg string) {
	http.Error(ctx.Resp, msg, status)
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
	PreviewArtifactContent(ctx, path, buf)
}

func PreviewArtifactContent(ctx *context_module.Context, path string, reader io.ReadSeeker) {
	// seekable sources are served straight from storage, so the size limit has to be enforced here too
	size, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		log.Error("artifact preview Seek: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to read artifact")
		return
	}
	if size > setting.UI.MaxDisplayFileSize {
		WritePreviewRawError(ctx, http.StatusRequestEntityTooLarge, "file is too large to preview, please download the artifact instead")
		return
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		log.Error("artifact preview Seek: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to read artifact")
		return
	}

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

	opts := artifactPreviewServeHeaderOptions(path, st)
	if strings.HasPrefix(opts.ContentType, "text/html") {
		htmlReader, err := artifactPreviewHTMLReader(ctx, reader)
		if err != nil {
			log.Error("artifact preview HTML helper: %v", err)
			WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to read artifact")
			return
		}
		defer htmlReader.Close()
		ctx.ServeContent(htmlReader, opts)
		return
	}

	// CSP sandbox is applied by httplib.ServeSetHeaders, see HINT: PDF-RENDER-SANDBOX
	ctx.ServeContent(reader, opts)
}

func ArtifactsPreviewView(ctx *context_module.Context) {
	if setting.Actions.ArtifactPreviewMaxSize == 0 {
		ctx.NotFound(nil)
		return
	}
	artifactName := ctx.PathParam("artifact_name")

	run, artifacts, ok := getCurrentRunAndUploadedArtifacts(ctx, artifactName)
	if !ok {
		return
	}

	var artifactSize int64
	for _, artifact := range artifacts {
		artifactSize += artifact.FileSize
	}
	previewTooLarge := !isArtifactPreviewSizeAllowed(artifactSize)
	var list artifactPreviewList
	if !previewTooLarge {
		var err error
		list, err = listPreview(artifacts)
		if err != nil {
			ctx.ServerError("listPreview", err)
			return
		}
	}
	requested := normalizeArtifactPreviewPath(ctx.FormString("path"))
	selectedPath := ChoosePreviewPath(list.paths, requested)
	if selectedPath == "" && requested != "" && list.truncated {
		// the listing was capped, so absence from it does not prove the file is missing: select it and let the raw view report the real result
		selectedPath = requested
	}
	PrepareArtifactPreviewTemplateData(ctx, run.Link(), run.Index, ctx.FormInt64("attempt"), artifactName, requested, selectedPath, list.paths, previewTooLarge, list.truncated)

	ctx.HTML(http.StatusOK, tplArtifactPreviewAction)
}

// serveArtifactV4PreviewRaw opens the v4 artifact zip once and serves a single file from it,
// avoiding the redundant parse that listPreview would do for raw fetches.
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
		PreviewArtifactContent(ctx, selectedPath, f)
		return
	}
	defer obj.Close()

	zf := findArtifactV4ZipFile(reader, requested)
	if zf == nil {
		WritePreviewRawError(ctx, http.StatusNotFound, "artifact file not found")
		return
	}
	r, err := zf.Open()
	if err != nil {
		log.Error("artifact preview zip.File.Open: %v", err)
		WritePreviewRawError(ctx, http.StatusInternalServerError, "failed to open artifact file")
		return
	}
	defer r.Close()
	previewArtifactByReader(ctx, requested, r)
}

func ArtifactsPreviewRawView(ctx *context_module.Context) {
	if setting.Actions.ArtifactPreviewMaxSize == 0 {
		ctx.NotFound(nil)
		return
	}
	artifactName := ctx.PathParam("artifact_name")

	_, artifacts, ok := getCurrentRunAndUploadedArtifacts(ctx, artifactName)
	if !ok {
		return
	}
	var artifactSize int64
	for _, artifact := range artifacts {
		artifactSize += artifact.FileSize
	}
	if !isArtifactPreviewSizeAllowed(artifactSize) {
		WritePreviewRawError(ctx, http.StatusRequestEntityTooLarge, "artifact is too large to preview, please download it instead")
		return
	}
	requested := normalizeArtifactPreviewPath(strings.TrimPrefix(ctx.PathParam("*"), "/"))

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

	PreviewArtifactContent(ctx, selectedPath, f)
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
	artifactName := ctx.PathParam("artifact_name")
	_, artifacts, ok := getCurrentRunAndUploadedArtifacts(ctx, artifactName)
	if !ok {
		return
	}

	// A v4 Artifact may only contain a single file
	// Multiple files are uploaded as a single file archive
	// All other cases fall back to the legacy v1–v3 zip handling below
	if len(artifacts) == 1 && actions_service.IsArtifactV4(artifacts[0]) {
		err := actions_service.DownloadArtifactV4(ctx.Base, artifacts[0])
		if err != nil {
			ctx.ServerError("DownloadArtifactV4", err)
			return
		}
		return
	}

	ctx.Resp.Header().Set("Content-Disposition", httplib.EncodeContentDispositionAttachment(artifactName+".zip"))

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

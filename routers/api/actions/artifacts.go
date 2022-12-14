package actions

import (
	gocontext "context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
)

const artifactRouteBase = "/_apis/pipelines/workflows/{taskID}/artifacts"

func ArtifactsRoutes(goctx gocontext.Context, prefix string) *web.Route {
	m := web.NewRoute()
	m.Use(withContexter(goctx))

	r := artifactRoutes{prefix: prefix, fs: actions.NewDiskMkdirFs("./artifacts-data")}

	// retrieve, list and confirm artifacts
	m.Post(artifactRouteBase, r.getUploadArtifactURL)
	m.Get(artifactRouteBase, r.listArtifacts)
	m.Patch(artifactRouteBase, r.patchArtifact)

	// handle container artifacts
	m.Put(artifactRouteBase+"/{artifactID}/upload", r.uploadArtifact)
	m.Get(artifactRouteBase+"/{artifactID}/path", r.getDownloadArtifactURL)
	m.Get(artifactRouteBase+"/{artifactID}/download", r.downloadArtifact)

	return m
}

// PackageContexter initializes a package context for a request.
func withContexter(ctx gocontext.Context) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := context.Context{
				Resp: context.NewResponse(resp),
				Data: map[string]interface{}{},
			}
			defer ctx.Close()

			ctx.Req = context.WithContext(req, &ctx)

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

type artifactRoutes struct {
	prefix string
	fs     actions.MkdirFS
}

func (ar artifactRoutes) openFile(fpath string, contentRange string) (actions.ArtifactFile, bool, error) {
	if contentRange != "" && !strings.HasPrefix(contentRange, "bytes 0-") {
		f, err := ar.fs.OpenAtEnd(fpath)
		return f, true, err
	}
	f, err := ar.fs.Open(fpath)
	return f, false, err
}

func (ar artifactRoutes) buildArtifactUrl(taskID int64, artifactID int64, suffix string) (string, error) {
	uploadURL := strings.TrimSuffix(setting.AppURL, "/") + strings.TrimSuffix(ar.prefix, "/") +
		strings.ReplaceAll(artifactRouteBase, "{taskID}", strconv.FormatInt(taskID, 10)) +
		"/" + strconv.FormatInt(artifactID, 10) + "/" + suffix
	u, err := url.Parse(uploadURL)
	if err != nil {
		return "", err
	}
	// FIXME: fix localhost ?? act starts docker container to run action.
	// It can't visit host network in container.
	if strings.Contains(u.Host, "localhost") {
		u.Host = strings.ReplaceAll(u.Host, "localhost", actions.GetOutboundIP().String())
	}
	if strings.Contains(u.Host, "127.0.0.1") {
		u.Host = strings.ReplaceAll(u.Host, "127.0.0.1", actions.GetOutboundIP().String())
	}
	return u.String(), nil
}

// getUploadArtifactURL generates a URL for uploading an artifact
func (ar artifactRoutes) getUploadArtifactURL(ctx *context.Context) {
	// get task
	taskID := ctx.ParamsInt64("taskID")
	task, err := actions.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Error("Error getting task: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	artifact, err := actions.CreateArtifact(ctx, task)
	if err != nil {
		log.Error("Error creating artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	url, err := ar.buildArtifactUrl(taskID, artifact.ID, "upload")
	if err != nil {
		log.Error("Error parsing upload URL: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"fileContainerResourceUrl": url,
	})
}

func (ar artifactRoutes) uploadArtifact(ctx *context.Context) {
	artifactID := ctx.ParamsInt64("artifactID")

	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	itemPath := ctx.Req.URL.Query().Get("itemPath")
	taskID := ctx.Params("taskID")
	artifactName := strings.Split(itemPath, "/")[0]

	if ctx.Req.Header.Get("Content-Encoding") == "gzip" {
		itemPath += ".gz"
	}
	filePath := fmt.Sprintf("%s/%d/%s", taskID, artifactID, itemPath)

	fSize := int64(0)
	file, isChunked, err := ar.openFile(filePath, ctx.Req.Header.Get("Content-Range"))
	if err != nil {
		log.Error("Error opening file: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	if isChunked {
		// chunked means it is a continuation of a previous upload
		fSize = artifact.FileSize
	}

	n, err := io.Copy(file, ctx.Req.Body)
	if err != nil {
		log.Error("Error copying body to file: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	fSize += n
	artifact.FilePath = filePath // path in storage
	artifact.ArtifactName = artifactName
	artifact.ArtifactPath = itemPath // path in container
	artifact.FileSize = fSize

	if err := actions.UpdateArtifactByID(ctx, artifact.ID, artifact); err != nil {
		log.Error("Error updating artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(200, map[string]string{
		"message": "success",
	})
}

// TODO: why it is used? confirm artifact uploaded successfully?
func (ar artifactRoutes) patchArtifact(ctx *context.Context) {
	ctx.JSON(200, map[string]string{
		"message": "success",
	})
}

func (ar artifactRoutes) listArtifacts(ctx *context.Context) {
	// get task
	taskID := ctx.ParamsInt64("taskID")
	task, err := actions.GetTaskByID(ctx, taskID)
	if err != nil {
		log.Error("Error getting task: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	artficats, err := actions.ListArtifactByJobID(ctx, task.JobID)
	if err != nil {
		log.Error("Error getting artifacts: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	artficatsData := make([]map[string]interface{}, 0, len(artficats))
	for _, a := range artficats {
		url, err := ar.buildArtifactUrl(taskID, a.ID, "path")
		if err != nil {
			log.Error("Error parsing artifact URL: %v", err)
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		artficatsData = append(artficatsData, map[string]interface{}{
			"name":                     a.ArtifactName,
			"fileContainerResourceUrl": url,
		})
	}
	respData := map[string]interface{}{
		"count": len(artficatsData),
		"value": artficatsData,
	}
	ctx.JSON(200, respData)
}

func (ar artifactRoutes) getDownloadArtifactURL(ctx *context.Context) {
	artifactID := ctx.ParamsInt64("artifactID")
	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	taskID := ctx.ParamsInt64("taskID")
	url, err := ar.buildArtifactUrl(taskID, artifact.ID, "download")
	if err != nil {
		log.Error("Error parsing download URL: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	itemPath := ctx.Req.URL.Query().Get("itemPath")
	artifactData := map[string]string{
		"path":            filepath.Join(itemPath, artifact.ArtifactPath),
		"itemType":        "file",
		"contentLocation": url,
	}
	respData := map[string]interface{}{
		"value": []interface{}{artifactData},
	}
	ctx.JSON(200, respData)
}

func (ar artifactRoutes) downloadArtifact(ctx *context.Context) {
	artifactID := ctx.ParamsInt64("artifactID")
	artifact, err := actions.GetArtifactByID(ctx, artifactID)
	if err != nil {
		log.Error("Error getting artifact: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	taskID := ctx.ParamsInt64("taskID")
	if artifact.JobID != taskID {
		log.Error("Error dismatch taskID and artifactID, task: %v, artifact: %v", taskID, artifactID)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	fd, err := ar.fs.Open(artifact.FilePath)
	if err != nil {
		log.Error("Error opening file: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	if strings.HasSuffix(artifact.ArtifactPath, ".gz") {
		ctx.Resp.Header().Set("Content-Encoding", "gzip")
	}
	_, err = io.Copy(ctx.Resp, fd)
	if err != nil {
		log.Error("Error copying file to response: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
}

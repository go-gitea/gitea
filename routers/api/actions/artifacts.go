package actions

import (
	gocontext "context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
)

func ArtifactsRoutes(goctx gocontext.Context, prefix string) *web.Route {
	m := web.NewRoute()
	m.Use(withContexter(goctx))

	routes := artifactRoutes{prefix: prefix, fs: actions.NewDiskMkdirFs("./artifacts-data")}

	m.Post("/_apis/pipelines/workflows/{runId}/artifacts", routes.getUploadArtifactURL)
	m.Put("/_apis/pipelines/workflows/{runId}/artifacts/{artifactID}/upload", routes.uploadArtifact)
	m.Patch("/_apis/pipelines/workflows/{runId}/artifacts", routes.patchArtifact)
	m.Get("/_apis/pipelines/workflows/{runId}/artifacts", listJobArtifacts)
	m.Get("/download/:container", listContainerArtifacts)
	m.Get("/artifact/:path", downloadArtifact)
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

// getUploadArtifactURL generates a URL for uploading an artifact
func (ar artifactRoutes) getUploadArtifactURL(ctx *context.Context) {
	// get task
	jobID := ctx.ParamsInt64("runId")

	task, err := actions.GetTaskByID(ctx, jobID)
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

	uploadURL := strings.TrimSuffix(setting.AppURL, "/") +
		ctx.Req.URL.Path + "/" + strconv.FormatInt(artifact.ID, 10) + "/upload"

	u, err := url.Parse(uploadURL)
	if err != nil {
		log.Error("Error parsing upload URL: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	// FIXME: fix localhost ?? act starts docker container to run action.
	// It can't visit host network in container.
	if strings.Contains(u.Host, "localhost") {
		u.Host = strings.ReplaceAll(u.Host, "localhost", actions.GetOutboundIP().String())
	}
	if strings.Contains(u.Host, "127.0.0.1") {
		u.Host = strings.ReplaceAll(u.Host, "127.0.0.1", actions.GetOutboundIP().String())
	}
	ctx.JSON(200, map[string]interface{}{
		"fileContainerResourceUrl": u.String(),
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
	runID := ctx.Params("runId")
	artifactName := strings.Split(itemPath, "/")[0]

	if ctx.Req.Header.Get("Content-Encoding") == "gzip" {
		itemPath += ".gz"
	}
	filePath := fmt.Sprintf("%s/%d/%s", runID, artifactID, itemPath)

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

// TODO: why it is used?
func (ar artifactRoutes) patchArtifact(ctx *context.Context) {

	ctx.JSON(200, map[string]string{
		"message": "success",
	})
}

func listJobArtifacts(ctx *context.Context) {

}

func listContainerArtifacts(ctx *context.Context) {

}

func downloadArtifact(ctx *context.Context) {

}

package actions

import (
	gocontext "context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (ar artifactRoutes) openFile(fpath string, contentRange string) (actions.ArtifactFile, error) {
	if contentRange != "" && !strings.HasPrefix(contentRange, "bytes 0-") {
		return ar.fs.OpenAtEnd(fpath)
	}
	return ar.fs.Open(fpath)
}

// getUploadArtifactURL generates a URL for uploading an artifact
func (ar artifactRoutes) getUploadArtifactURL(ctx *context.Context) {
	jobID := ctx.Params("runId")
	uploadURL := strings.TrimSuffix(setting.AppURL, "/") + ctx.Req.URL.Path + "/" + jobID + "/upload"
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
	itemPath := ctx.Req.URL.Query().Get("itemPath")
	runID := ctx.Params("runId")

	if ctx.Req.Header.Get("Content-Encoding") == "gzip" {
		itemPath += ".gz"
	}
	filePath := fmt.Sprintf("%s/%s", runID, itemPath)

	file, err := ar.openFile(filePath, ctx.Req.Header.Get("Content-Range"))
	if err != nil {
		log.Error("Error opening file: %v", err)
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	_, err = io.Copy(file, ctx.Req.Body)
	if err != nil {
		log.Error("Error copying body to file: %v", err)
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

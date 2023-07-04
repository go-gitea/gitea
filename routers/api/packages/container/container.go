// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package container

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	container_module "code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
	container_service "code.gitea.io/gitea/services/packages/container"

	digest "github.com/opencontainers/go-digest"
)

// maximum size of a container manifest
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pushing-manifests
const maxManifestSize = 10 * 1024 * 1024

var (
	imageNamePattern = regexp.MustCompile(`\A[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*\z`)
	referencePattern = regexp.MustCompile(`\A[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}\z`)
)

type containerHeaders struct {
	Status        int
	ContentDigest string
	UploadUUID    string
	Range         string
	Location      string
	ContentType   string
	ContentLength int64
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#legacy-docker-support-http-headers
func setResponseHeaders(resp http.ResponseWriter, h *containerHeaders) {
	if h.Location != "" {
		resp.Header().Set("Location", h.Location)
	}
	if h.Range != "" {
		resp.Header().Set("Range", h.Range)
	}
	if h.ContentType != "" {
		resp.Header().Set("Content-Type", h.ContentType)
	}
	if h.ContentLength != 0 {
		resp.Header().Set("Content-Length", strconv.FormatInt(h.ContentLength, 10))
	}
	if h.UploadUUID != "" {
		resp.Header().Set("Docker-Upload-Uuid", h.UploadUUID)
	}
	if h.ContentDigest != "" {
		resp.Header().Set("Docker-Content-Digest", h.ContentDigest)
		resp.Header().Set("ETag", fmt.Sprintf(`"%s"`, h.ContentDigest))
	}
	resp.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	resp.WriteHeader(h.Status)
}

func jsonResponse(ctx *context.Context, status int, obj any) {
	setResponseHeaders(ctx.Resp, &containerHeaders{
		Status:      status,
		ContentType: "application/json",
	})
	if err := json.NewEncoder(ctx.Resp).Encode(obj); err != nil {
		log.Error("JSON encode: %v", err)
	}
}

func apiError(ctx *context.Context, status int, err error) {
	helper.LogAndProcessError(ctx, status, err, func(message string) {
		setResponseHeaders(ctx.Resp, &containerHeaders{
			Status: status,
		})
	})
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#error-codes
func apiErrorDefined(ctx *context.Context, err *namedError) {
	type ContainerError struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	type ContainerErrors struct {
		Errors []ContainerError `json:"errors"`
	}

	jsonResponse(ctx, err.StatusCode, ContainerErrors{
		Errors: []ContainerError{
			{
				Code:    err.Code,
				Message: err.Message,
			},
		},
	})
}

// ReqContainerAccess is a middleware which checks the current user valid (real user or ghost for anonymous access)
func ReqContainerAccess(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.Resp.Header().Add("WWW-Authenticate", `Bearer realm="`+setting.AppURL+`v2/token",service="container_registry",scope="*"`)
		apiErrorDefined(ctx, errUnauthorized)
	}
}

// VerifyImageName is a middleware which checks if the image name is allowed
func VerifyImageName(ctx *context.Context) {
	if !imageNamePattern.MatchString(ctx.Params("image")) {
		apiErrorDefined(ctx, errNameInvalid)
	}
}

// DetermineSupport is used to test if the registry supports OCI
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#determining-support
func DetermineSupport(ctx *context.Context) {
	setResponseHeaders(ctx.Resp, &containerHeaders{
		Status: http.StatusOK,
	})
}

// Authenticate creates a token for the current user
// If the current user is anonymous, the ghost user is used
func Authenticate(ctx *context.Context) {
	u := ctx.Doer
	if u == nil {
		u = user_model.NewGhostUser()
	}

	token, err := packages_service.CreateAuthorizationToken(u)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"token": token,
	})
}

// https://docs.docker.com/registry/spec/api/#listing-repositories
func GetRepositoryList(ctx *context.Context) {
	n := ctx.FormInt("n")
	if n <= 0 || n > 100 {
		n = 100
	}
	last := ctx.FormTrim("last")

	repositories, err := container_model.GetRepositories(ctx, ctx.Doer, n, last)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	type RepositoryList struct {
		Repositories []string `json:"repositories"`
	}

	if len(repositories) == n {
		v := url.Values{}
		if n > 0 {
			v.Add("n", strconv.Itoa(n))
		}
		v.Add("last", repositories[len(repositories)-1])

		ctx.Resp.Header().Set("Link", fmt.Sprintf(`</v2/_catalog?%s>; rel="next"`, v.Encode()))
	}

	jsonResponse(ctx, http.StatusOK, RepositoryList{
		Repositories: repositories,
	})
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#mounting-a-blob-from-another-repository
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#single-post
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pushing-a-blob-in-chunks
func InitiateUploadBlob(ctx *context.Context) {
	image := ctx.Params("image")

	mount := ctx.FormTrim("mount")
	from := ctx.FormTrim("from")
	if mount != "" {
		blob, _ := workaroundGetContainerBlob(ctx, &container_model.BlobSearchOptions{
			Repository: from,
			Digest:     mount,
		})
		if blob != nil {
			if err := mountBlob(&packages_service.PackageInfo{Owner: ctx.Package.Owner, Name: image}, blob.Blob); err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}

			setResponseHeaders(ctx.Resp, &containerHeaders{
				Location:      fmt.Sprintf("/v2/%s/%s/blobs/%s", ctx.Package.Owner.LowerName, image, mount),
				ContentDigest: mount,
				Status:        http.StatusCreated,
			})
			return
		}
	}

	digest := ctx.FormTrim("digest")
	if digest != "" {
		buf, err := packages_module.CreateHashedBufferFromReader(ctx.Req.Body)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		defer buf.Close()

		if digest != digestFromHashSummer(buf) {
			apiErrorDefined(ctx, errDigestInvalid)
			return
		}

		if _, err := saveAsPackageBlob(
			buf,
			&packages_service.PackageCreationInfo{
				PackageInfo: packages_service.PackageInfo{
					Owner: ctx.Package.Owner,
					Name:  image,
				},
				Creator: ctx.Doer,
			},
		); err != nil {
			switch err {
			case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
				apiError(ctx, http.StatusForbidden, err)
			default:
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}

		setResponseHeaders(ctx.Resp, &containerHeaders{
			Location:      fmt.Sprintf("/v2/%s/%s/blobs/%s", ctx.Package.Owner.LowerName, image, digest),
			ContentDigest: digest,
			Status:        http.StatusCreated,
		})
		return
	}

	upload, err := packages_model.CreateBlobUpload(ctx)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		Location:   fmt.Sprintf("/v2/%s/%s/blobs/uploads/%s", ctx.Package.Owner.LowerName, image, upload.ID),
		Range:      "0-0",
		UploadUUID: upload.ID,
		Status:     http.StatusAccepted,
	})
}

// https://docs.docker.com/registry/spec/api/#get-blob-upload
func GetUploadBlob(ctx *context.Context) {
	uuid := ctx.Params("uuid")

	upload, err := packages_model.GetBlobUploadByID(ctx, uuid)
	if err != nil {
		if err == packages_model.ErrPackageBlobUploadNotExist {
			apiErrorDefined(ctx, errBlobUploadUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		Range:      fmt.Sprintf("0-%d", upload.BytesReceived),
		UploadUUID: upload.ID,
		Status:     http.StatusNoContent,
	})
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pushing-a-blob-in-chunks
func UploadBlob(ctx *context.Context) {
	image := ctx.Params("image")

	uploader, err := container_service.NewBlobUploader(ctx, ctx.Params("uuid"))
	if err != nil {
		if err == packages_model.ErrPackageBlobUploadNotExist {
			apiErrorDefined(ctx, errBlobUploadUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	defer uploader.Close()

	contentRange := ctx.Req.Header.Get("Content-Range")
	if contentRange != "" {
		start, end := 0, 0
		if _, err := fmt.Sscanf(contentRange, "%d-%d", &start, &end); err != nil {
			apiErrorDefined(ctx, errBlobUploadInvalid)
			return
		}

		if int64(start) != uploader.Size() {
			apiErrorDefined(ctx, errBlobUploadInvalid.WithStatusCode(http.StatusRequestedRangeNotSatisfiable))
			return
		}
	} else if uploader.Size() != 0 {
		apiErrorDefined(ctx, errBlobUploadInvalid.WithMessage("Stream uploads after first write are not allowed"))
		return
	}

	if err := uploader.Append(ctx, ctx.Req.Body); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		Location:   fmt.Sprintf("/v2/%s/%s/blobs/uploads/%s", ctx.Package.Owner.LowerName, image, uploader.ID),
		Range:      fmt.Sprintf("0-%d", uploader.Size()-1),
		UploadUUID: uploader.ID,
		Status:     http.StatusAccepted,
	})
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pushing-a-blob-in-chunks
func EndUploadBlob(ctx *context.Context) {
	image := ctx.Params("image")

	digest := ctx.FormTrim("digest")
	if digest == "" {
		apiErrorDefined(ctx, errDigestInvalid)
		return
	}

	uploader, err := container_service.NewBlobUploader(ctx, ctx.Params("uuid"))
	if err != nil {
		if err == packages_model.ErrPackageBlobUploadNotExist {
			apiErrorDefined(ctx, errBlobUploadUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	close := true
	defer func() {
		if close {
			uploader.Close()
		}
	}()

	if ctx.Req.Body != nil {
		if err := uploader.Append(ctx, ctx.Req.Body); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	if digest != digestFromHashSummer(uploader) {
		apiErrorDefined(ctx, errDigestInvalid)
		return
	}

	if _, err := saveAsPackageBlob(
		uploader,
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner: ctx.Package.Owner,
				Name:  image,
			},
			Creator: ctx.Doer,
		},
	); err != nil {
		switch err {
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if err := uploader.Close(); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	close = false

	if err := container_service.RemoveBlobUploadByID(ctx, uploader.ID); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		Location:      fmt.Sprintf("/v2/%s/%s/blobs/%s", ctx.Package.Owner.LowerName, image, digest),
		ContentDigest: digest,
		Status:        http.StatusCreated,
	})
}

// https://docs.docker.com/registry/spec/api/#delete-blob-upload
func CancelUploadBlob(ctx *context.Context) {
	uuid := ctx.Params("uuid")

	_, err := packages_model.GetBlobUploadByID(ctx, uuid)
	if err != nil {
		if err == packages_model.ErrPackageBlobUploadNotExist {
			apiErrorDefined(ctx, errBlobUploadUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	if err := container_service.RemoveBlobUploadByID(ctx, uuid); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		Status: http.StatusNoContent,
	})
}

func getBlobFromContext(ctx *context.Context) (*packages_model.PackageFileDescriptor, error) {
	d := ctx.Params("digest")

	if digest.Digest(d).Validate() != nil {
		return nil, container_model.ErrContainerBlobNotExist
	}

	return workaroundGetContainerBlob(ctx, &container_model.BlobSearchOptions{
		OwnerID: ctx.Package.Owner.ID,
		Image:   ctx.Params("image"),
		Digest:  d,
	})
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#checking-if-content-exists-in-the-registry
func HeadBlob(ctx *context.Context) {
	blob, err := getBlobFromContext(ctx)
	if err != nil {
		if err == container_model.ErrContainerBlobNotExist {
			apiErrorDefined(ctx, errBlobUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		ContentDigest: blob.Properties.GetByName(container_module.PropertyDigest),
		ContentLength: blob.Blob.Size,
		Status:        http.StatusOK,
	})
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-blobs
func GetBlob(ctx *context.Context) {
	blob, err := getBlobFromContext(ctx)
	if err != nil {
		if err == container_model.ErrContainerBlobNotExist {
			apiErrorDefined(ctx, errBlobUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	serveBlob(ctx, blob)
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#deleting-blobs
func DeleteBlob(ctx *context.Context) {
	d := ctx.Params("digest")

	if digest.Digest(d).Validate() != nil {
		apiErrorDefined(ctx, errBlobUnknown)
		return
	}

	if err := deleteBlob(ctx.Package.Owner.ID, ctx.Params("image"), d); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		Status: http.StatusAccepted,
	})
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pushing-manifests
func UploadManifest(ctx *context.Context) {
	reference := ctx.Params("reference")

	mci := &manifestCreationInfo{
		MediaType: ctx.Req.Header.Get("Content-Type"),
		Owner:     ctx.Package.Owner,
		Creator:   ctx.Doer,
		Image:     ctx.Params("image"),
		Reference: reference,
		IsTagged:  digest.Digest(reference).Validate() != nil,
	}

	if mci.IsTagged && !referencePattern.MatchString(reference) {
		apiErrorDefined(ctx, errManifestInvalid.WithMessage("Tag is invalid"))
		return
	}

	maxSize := maxManifestSize + 1
	buf, err := packages_module.CreateHashedBufferFromReaderWithSize(&io.LimitedReader{R: ctx.Req.Body, N: int64(maxSize)}, maxSize)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	if buf.Size() > maxManifestSize {
		apiErrorDefined(ctx, errManifestInvalid.WithMessage("Manifest exceeds maximum size").WithStatusCode(http.StatusRequestEntityTooLarge))
		return
	}

	digest, err := processManifest(mci, buf)
	if err != nil {
		var namedError *namedError
		if errors.As(err, &namedError) {
			apiErrorDefined(ctx, namedError)
		} else if errors.Is(err, container_model.ErrContainerBlobNotExist) {
			apiErrorDefined(ctx, errBlobUnknown)
		} else {
			switch err {
			case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
				apiError(ctx, http.StatusForbidden, err)
			default:
				apiError(ctx, http.StatusInternalServerError, err)
			}
		}
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		Location:      fmt.Sprintf("/v2/%s/%s/manifests/%s", ctx.Package.Owner.LowerName, mci.Image, reference),
		ContentDigest: digest,
		Status:        http.StatusCreated,
	})
}

func getBlobSearchOptionsFromContext(ctx *context.Context) (*container_model.BlobSearchOptions, error) {
	reference := ctx.Params("reference")

	opts := &container_model.BlobSearchOptions{
		OwnerID:    ctx.Package.Owner.ID,
		Image:      ctx.Params("image"),
		IsManifest: true,
	}

	if digest.Digest(reference).Validate() == nil {
		opts.Digest = reference
	} else if referencePattern.MatchString(reference) {
		opts.Tag = reference
	} else {
		return nil, container_model.ErrContainerBlobNotExist
	}

	return opts, nil
}

func getManifestFromContext(ctx *context.Context) (*packages_model.PackageFileDescriptor, error) {
	opts, err := getBlobSearchOptionsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return workaroundGetContainerBlob(ctx, opts)
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#checking-if-content-exists-in-the-registry
func HeadManifest(ctx *context.Context) {
	manifest, err := getManifestFromContext(ctx)
	if err != nil {
		if err == container_model.ErrContainerBlobNotExist {
			apiErrorDefined(ctx, errManifestUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		ContentDigest: manifest.Properties.GetByName(container_module.PropertyDigest),
		ContentType:   manifest.Properties.GetByName(container_module.PropertyMediaType),
		ContentLength: manifest.Blob.Size,
		Status:        http.StatusOK,
	})
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
func GetManifest(ctx *context.Context) {
	manifest, err := getManifestFromContext(ctx)
	if err != nil {
		if err == container_model.ErrContainerBlobNotExist {
			apiErrorDefined(ctx, errManifestUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	serveBlob(ctx, manifest)
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#deleting-tags
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#deleting-manifests
func DeleteManifest(ctx *context.Context) {
	opts, err := getBlobSearchOptionsFromContext(ctx)
	if err != nil {
		apiErrorDefined(ctx, errManifestUnknown)
		return
	}

	pvs, err := container_model.GetManifestVersions(ctx, opts)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pvs) == 0 {
		apiErrorDefined(ctx, errManifestUnknown)
		return
	}

	for _, pv := range pvs {
		if err := packages_service.RemovePackageVersion(ctx.Doer, pv); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	setResponseHeaders(ctx.Resp, &containerHeaders{
		Status: http.StatusAccepted,
	})
}

func serveBlob(ctx *context.Context, pfd *packages_model.PackageFileDescriptor) {
	s, u, _, err := packages_service.GetPackageBlobStream(ctx, pfd.File, pfd.Blob)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	headers := &containerHeaders{
		ContentDigest: pfd.Properties.GetByName(container_module.PropertyDigest),
		ContentType:   pfd.Properties.GetByName(container_module.PropertyMediaType),
		ContentLength: pfd.Blob.Size,
		Status:        http.StatusOK,
	}

	if u != nil {
		headers.Status = http.StatusTemporaryRedirect
		headers.Location = u.String()

		setResponseHeaders(ctx.Resp, headers)
		return
	}

	defer s.Close()

	setResponseHeaders(ctx.Resp, headers)
	if _, err := io.Copy(ctx.Resp, s); err != nil {
		log.Error("Error whilst copying content to response: %v", err)
	}
}

// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#content-discovery
func GetTagList(ctx *context.Context) {
	image := ctx.Params("image")

	if _, err := packages_model.GetPackageByName(ctx, ctx.Package.Owner.ID, packages_model.TypeContainer, image); err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiErrorDefined(ctx, errNameUnknown)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	n := -1
	if ctx.FormTrim("n") != "" {
		n = ctx.FormInt("n")
	}
	last := ctx.FormTrim("last")

	tags, err := container_model.GetImageTags(ctx, ctx.Package.Owner.ID, image, n, last)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	type TagList struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	if len(tags) > 0 {
		v := url.Values{}
		if n > 0 {
			v.Add("n", strconv.Itoa(n))
		}
		v.Add("last", tags[len(tags)-1])

		ctx.Resp.Header().Set("Link", fmt.Sprintf(`</v2/%s/%s/tags/list?%s>; rel="next"`, ctx.Package.Owner.LowerName, image, v.Encode()))
	}

	jsonResponse(ctx, http.StatusOK, TagList{
		Name: strings.ToLower(ctx.Package.Owner.LowerName + "/" + image),
		Tags: tags,
	})
}

// FIXME: Workaround to be removed in v1.20
// https://github.com/go-gitea/gitea/issues/19586
func workaroundGetContainerBlob(ctx *context.Context, opts *container_model.BlobSearchOptions) (*packages_model.PackageFileDescriptor, error) {
	blob, err := container_model.GetContainerBlob(ctx, opts)
	if err != nil {
		return nil, err
	}

	err = packages_module.NewContentStore().Has(packages_module.BlobHash256Key(blob.Blob.HashSHA256))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			log.Debug("Package registry inconsistent: blob %s does not exist on file system", blob.Blob.HashSHA256)
			return nil, container_model.ErrContainerBlobNotExist
		}
		return nil, err
	}

	return blob, nil
}

// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conan

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	conan_model "code.gitea.io/gitea/models/packages/conan"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	packages_module "code.gitea.io/gitea/modules/packages"
	conan_module "code.gitea.io/gitea/modules/packages/conan"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
)

const (
	conanfileFile = "conanfile.py"
	conaninfoFile = "conaninfo.txt"

	recipeReferenceKey  = "RecipeReference"
	packageReferenceKey = "PackageReference"
)

var (
	recipeFileList = container.SetOf(
		conanfileFile,
		"conanmanifest.txt",
		"conan_sources.tgz",
		"conan_export.tgz",
	)
	packageFileList = container.SetOf(
		conaninfoFile,
		"conanmanifest.txt",
		"conan_package.tgz",
	)
)

func jsonResponse(ctx *context.Context, status int, obj any) {
	// https://github.com/conan-io/conan/issues/6613
	ctx.Resp.Header().Set("Content-Type", "application/json")
	ctx.Status(status)
	if err := json.NewEncoder(ctx.Resp).Encode(obj); err != nil {
		log.Error("JSON encode: %v", err)
	}
}

func apiError(ctx *context.Context, status int, obj any) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		jsonResponse(ctx, status, map[string]string{
			"message": message,
		})
	})
}

func baseURL(ctx *context.Context) string {
	return setting.AppURL + "api/packages/" + ctx.Package.Owner.Name + "/conan"
}

// ExtractPathParameters is a middleware to extract common parameters from path
func ExtractPathParameters(ctx *context.Context) {
	rref, err := conan_module.NewRecipeReference(
		ctx.Params("name"),
		ctx.Params("version"),
		ctx.Params("user"),
		ctx.Params("channel"),
		ctx.Params("recipe_revision"),
	)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	ctx.Data[recipeReferenceKey] = rref

	reference := ctx.Params("package_reference")

	var pref *conan_module.PackageReference
	if reference != "" {
		pref, err = conan_module.NewPackageReference(
			rref,
			reference,
			ctx.Params("package_revision"),
		)
		if err != nil {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
	}

	ctx.Data[packageReferenceKey] = pref
}

// Ping reports the server capabilities
func Ping(ctx *context.Context) {
	ctx.RespHeader().Add("X-Conan-Server-Capabilities", "revisions") // complex_search,checksum_deploy,matrix_params

	ctx.Status(http.StatusOK)
}

// Authenticate creates an authentication token for the user
func Authenticate(ctx *context.Context) {
	if ctx.Doer == nil {
		apiError(ctx, http.StatusBadRequest, nil)
		return
	}

	token, err := packages_service.CreateAuthorizationToken(ctx.Doer)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusOK, token)
}

// CheckCredentials tests if the provided authentication token is valid
func CheckCredentials(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.Status(http.StatusUnauthorized)
	} else {
		ctx.Status(http.StatusOK)
	}
}

// RecipeSnapshot displays the recipe files with their md5 hash
func RecipeSnapshot(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	serveSnapshot(ctx, rref.AsKey())
}

// RecipeSnapshot displays the package files with their md5 hash
func PackageSnapshot(ctx *context.Context) {
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	serveSnapshot(ctx, pref.AsKey())
}

func serveSnapshot(ctx *context.Context, fileKey string) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeConan, rref.Name, rref.Version)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		VersionID:    pv.ID,
		CompositeKey: fileKey,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pfs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	files := make(map[string]string)
	for _, pf := range pfs {
		pb, err := packages_model.GetBlobByID(ctx, pf.BlobID)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		files[pf.Name] = pb.HashMD5
	}

	jsonResponse(ctx, http.StatusOK, files)
}

// RecipeDownloadURLs displays the recipe files with their download url
func RecipeDownloadURLs(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	serveDownloadURLs(
		ctx,
		rref.AsKey(),
		fmt.Sprintf(baseURL(ctx)+"/v1/files/%s/recipe", rref.LinkName()),
	)
}

// PackageDownloadURLs displays the package files with their download url
func PackageDownloadURLs(ctx *context.Context) {
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	serveDownloadURLs(
		ctx,
		pref.AsKey(),
		fmt.Sprintf(baseURL(ctx)+"/v1/files/%s/package/%s", pref.Recipe.LinkName(), pref.LinkName()),
	)
}

func serveDownloadURLs(ctx *context.Context, fileKey, downloadURL string) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeConan, rref.Name, rref.Version)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		VersionID:    pv.ID,
		CompositeKey: fileKey,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pfs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	urls := make(map[string]string)
	for _, pf := range pfs {
		urls[pf.Name] = fmt.Sprintf("%s/%s", downloadURL, pf.Name)
	}

	jsonResponse(ctx, http.StatusOK, urls)
}

// RecipeUploadURLs displays the upload urls for the provided recipe files
func RecipeUploadURLs(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	serveUploadURLs(
		ctx,
		recipeFileList,
		fmt.Sprintf(baseURL(ctx)+"/v1/files/%s/recipe", rref.LinkName()),
	)
}

// PackageUploadURLs displays the upload urls for the provided package files
func PackageUploadURLs(ctx *context.Context) {
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	serveUploadURLs(
		ctx,
		packageFileList,
		fmt.Sprintf(baseURL(ctx)+"/v1/files/%s/package/%s", pref.Recipe.LinkName(), pref.LinkName()),
	)
}

func serveUploadURLs(ctx *context.Context, fileFilter container.Set[string], uploadURL string) {
	defer ctx.Req.Body.Close()

	var files map[string]int64
	if err := json.NewDecoder(ctx.Req.Body).Decode(&files); err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	urls := make(map[string]string)
	for file := range files {
		if fileFilter.Contains(file) {
			urls[file] = fmt.Sprintf("%s/%s", uploadURL, file)
		}
	}

	jsonResponse(ctx, http.StatusOK, urls)
}

// UploadRecipeFile handles the upload of a recipe file
func UploadRecipeFile(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	uploadFile(ctx, recipeFileList, rref.AsKey())
}

// UploadPackageFile handles the upload of a package file
func UploadPackageFile(ctx *context.Context) {
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	uploadFile(ctx, packageFileList, pref.AsKey())
}

func uploadFile(ctx *context.Context, fileFilter container.Set[string], fileKey string) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	filename := ctx.Params("filename")
	if !fileFilter.Contains(filename) {
		apiError(ctx, http.StatusBadRequest, nil)
		return
	}

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	if buf.Size() == 0 {
		// ignore empty uploads, second request contains content
		jsonResponse(ctx, http.StatusOK, nil)
		return
	}

	isConanfileFile := filename == conanfileFile

	pci := &packages_service.PackageCreationInfo{
		PackageInfo: packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeConan,
			Name:        rref.Name,
			Version:     rref.Version,
		},
		Creator: ctx.Doer,
	}
	pfci := &packages_service.PackageFileCreationInfo{
		PackageFileInfo: packages_service.PackageFileInfo{
			Filename:     strings.ToLower(filename),
			CompositeKey: fileKey,
		},
		Creator: ctx.Doer,
		Data:    buf,
		IsLead:  isConanfileFile,
		Properties: map[string]string{
			conan_module.PropertyRecipeUser:     rref.User,
			conan_module.PropertyRecipeChannel:  rref.Channel,
			conan_module.PropertyRecipeRevision: rref.RevisionOrDefault(),
		},
		OverwriteExisting: true,
	}

	if pref != nil {
		pfci.Properties[conan_module.PropertyPackageReference] = pref.Reference
		pfci.Properties[conan_module.PropertyPackageRevision] = pref.RevisionOrDefault()
	}

	if isConanfileFile || filename == conaninfoFile {
		if isConanfileFile {
			metadata, err := conan_module.ParseConanfile(buf)
			if err != nil {
				log.Error("Error parsing package metadata: %v", err)
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
			pv, err := packages_model.GetVersionByNameAndVersion(ctx, pci.Owner.ID, pci.PackageType, pci.Name, pci.Version)
			if err != nil && err != packages_model.ErrPackageNotExist {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
			if pv != nil {
				raw, err := json.Marshal(metadata)
				if err != nil {
					apiError(ctx, http.StatusInternalServerError, err)
					return
				}
				pv.MetadataJSON = string(raw)
				if err := packages_model.UpdateVersion(ctx, pv); err != nil {
					apiError(ctx, http.StatusInternalServerError, err)
					return
				}
			} else {
				pci.Metadata = metadata
			}
		} else {
			info, err := conan_module.ParseConaninfo(buf)
			if err != nil {
				log.Error("Error parsing conan info: %v", err)
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
			raw, err := json.Marshal(info)
			if err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
			pfci.Properties[conan_module.PropertyPackageInfo] = string(raw)
		}

		if _, err := buf.Seek(0, io.SeekStart); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		pci,
		pfci,
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusBadRequest, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	ctx.Status(http.StatusCreated)
}

// DownloadRecipeFile serves the content of the requested recipe file
func DownloadRecipeFile(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	downloadFile(ctx, recipeFileList, rref.AsKey())
}

// DownloadPackageFile serves the content of the requested package file
func DownloadPackageFile(ctx *context.Context) {
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	downloadFile(ctx, packageFileList, pref.AsKey())
}

func downloadFile(ctx *context.Context, fileFilter container.Set[string], fileKey string) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	filename := ctx.Params("filename")
	if !fileFilter.Contains(filename) {
		apiError(ctx, http.StatusBadRequest, nil)
		return
	}

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeConan,
			Name:        rref.Name,
			Version:     rref.Version,
		},
		&packages_service.PackageFileInfo{
			Filename:     filename,
			CompositeKey: fileKey,
		},
	)
	if err != nil {
		if err == packages_model.ErrPackageNotExist || err == packages_model.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer s.Close()

	ctx.ServeContent(s, &context.ServeHeaderOptions{
		Filename:     pf.Name,
		LastModified: pf.CreatedUnix.AsLocalTime(),
	})
}

// DeleteRecipeV1 deletes the requested recipe(s)
func DeleteRecipeV1(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	if err := deleteRecipeOrPackage(ctx, rref, true, nil, false); err != nil {
		if err == packages_model.ErrPackageNotExist || err == conan_model.ErrPackageReferenceNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	ctx.Status(http.StatusOK)
}

// DeleteRecipeV2 deletes the requested recipe(s) respecting its revisions
func DeleteRecipeV2(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	if err := deleteRecipeOrPackage(ctx, rref, rref.Revision == "", nil, false); err != nil {
		if err == packages_model.ErrPackageNotExist || err == conan_model.ErrPackageReferenceNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}
	ctx.Status(http.StatusOK)
}

// DeletePackageV1 deletes the requested package(s)
func DeletePackageV1(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	type PackageReferences struct {
		References []string `json:"package_ids"`
	}

	var ids *PackageReferences
	if err := json.NewDecoder(ctx.Req.Body).Decode(&ids); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	revisions, err := conan_model.GetRecipeRevisions(ctx, ctx.Package.Owner.ID, rref)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	for _, revision := range revisions {
		currentRref := rref.WithRevision(revision.Value)

		var references []*conan_model.PropertyValue
		if len(ids.References) == 0 {
			if references, err = conan_model.GetPackageReferences(ctx, ctx.Package.Owner.ID, currentRref); err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
		} else {
			for _, reference := range ids.References {
				references = append(references, &conan_model.PropertyValue{Value: reference})
			}
		}

		for _, reference := range references {
			pref, _ := conan_module.NewPackageReference(currentRref, reference.Value, conan_module.DefaultRevision)
			if err := deleteRecipeOrPackage(ctx, currentRref, true, pref, true); err != nil {
				if err == packages_model.ErrPackageNotExist || err == conan_model.ErrPackageReferenceNotExist {
					apiError(ctx, http.StatusNotFound, err)
				} else {
					apiError(ctx, http.StatusInternalServerError, err)
				}
				return
			}
		}
	}
	ctx.Status(http.StatusOK)
}

// DeletePackageV2 deletes the requested package(s) respecting its revisions
func DeletePackageV2(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	if pref != nil { // has package reference
		if err := deleteRecipeOrPackage(ctx, rref, false, pref, pref.Revision == ""); err != nil {
			if err == packages_model.ErrPackageNotExist || err == conan_model.ErrPackageReferenceNotExist {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
		} else {
			ctx.Status(http.StatusOK)
		}
		return
	}

	references, err := conan_model.GetPackageReferences(ctx, ctx.Package.Owner.ID, rref)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(references) == 0 {
		apiError(ctx, http.StatusNotFound, conan_model.ErrPackageReferenceNotExist)
		return
	}

	for _, reference := range references {
		pref, _ := conan_module.NewPackageReference(rref, reference.Value, conan_module.DefaultRevision)

		if err := deleteRecipeOrPackage(ctx, rref, false, pref, true); err != nil {
			if err == packages_model.ErrPackageNotExist || err == conan_model.ErrPackageReferenceNotExist {
				apiError(ctx, http.StatusNotFound, err)
			} else {
				apiError(ctx, http.StatusInternalServerError, err)
			}
			return
		}
	}

	ctx.Status(http.StatusOK)
}

func deleteRecipeOrPackage(apictx *context.Context, rref *conan_module.RecipeReference, ignoreRecipeRevision bool, pref *conan_module.PackageReference, ignorePackageRevision bool) error {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return err
	}
	defer committer.Close()

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, apictx.Package.Owner.ID, packages_model.TypeConan, rref.Name, rref.Version)
	if err != nil {
		return err
	}

	pd, err := packages_model.GetPackageDescriptor(ctx, pv)
	if err != nil {
		return err
	}

	filter := map[string]string{
		conan_module.PropertyRecipeUser:    rref.User,
		conan_module.PropertyRecipeChannel: rref.Channel,
	}
	if !ignoreRecipeRevision {
		filter[conan_module.PropertyRecipeRevision] = rref.RevisionOrDefault()
	}
	if pref != nil {
		filter[conan_module.PropertyPackageReference] = pref.Reference
		if !ignorePackageRevision {
			filter[conan_module.PropertyPackageRevision] = pref.RevisionOrDefault()
		}
	}

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		VersionID:  pv.ID,
		Properties: filter,
	})
	if err != nil {
		return err
	}
	if len(pfs) == 0 {
		return conan_model.ErrPackageReferenceNotExist
	}

	for _, pf := range pfs {
		if err := packages_service.DeletePackageFile(ctx, pf); err != nil {
			return err
		}
	}

	versionDeleted := false
	has, err := packages_model.HasVersionFileReferences(ctx, pv.ID)
	if err != nil {
		return err
	}
	if !has {
		versionDeleted = true

		if err := packages_service.DeletePackageVersionAndReferences(ctx, pv); err != nil {
			return err
		}
	}

	if err := committer.Commit(); err != nil {
		return err
	}

	if versionDeleted {
		notification.NotifyPackageDelete(apictx, apictx.Doer, pd)
	}

	return nil
}

// ListRecipeRevisions gets a list of all recipe revisions
func ListRecipeRevisions(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	revisions, err := conan_model.GetRecipeRevisions(ctx, ctx.Package.Owner.ID, rref)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	listRevisions(ctx, revisions)
}

// ListPackageRevisions gets a list of all package revisions
func ListPackageRevisions(ctx *context.Context) {
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	revisions, err := conan_model.GetPackageRevisions(ctx, ctx.Package.Owner.ID, pref)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	listRevisions(ctx, revisions)
}

type revisionInfo struct {
	Revision string    `json:"revision"`
	Time     time.Time `json:"time"`
}

func listRevisions(ctx *context.Context, revisions []*conan_model.PropertyValue) {
	if len(revisions) == 0 {
		apiError(ctx, http.StatusNotFound, conan_model.ErrRecipeReferenceNotExist)
		return
	}

	type RevisionList struct {
		Revisions []*revisionInfo `json:"revisions"`
	}

	revs := make([]*revisionInfo, 0, len(revisions))
	for _, rev := range revisions {
		revs = append(revs, &revisionInfo{Revision: rev.Value, Time: rev.CreatedUnix.AsLocalTime()})
	}

	jsonResponse(ctx, http.StatusOK, &RevisionList{revs})
}

// LatestRecipeRevision gets the latest recipe revision
func LatestRecipeRevision(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	revision, err := conan_model.GetLastRecipeRevision(ctx, ctx.Package.Owner.ID, rref)
	if err != nil {
		if err == conan_model.ErrRecipeReferenceNotExist || err == conan_model.ErrPackageReferenceNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	jsonResponse(ctx, http.StatusOK, &revisionInfo{Revision: revision.Value, Time: revision.CreatedUnix.AsLocalTime()})
}

// LatestPackageRevision gets the latest package revision
func LatestPackageRevision(ctx *context.Context) {
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	revision, err := conan_model.GetLastPackageRevision(ctx, ctx.Package.Owner.ID, pref)
	if err != nil {
		if err == conan_model.ErrRecipeReferenceNotExist || err == conan_model.ErrPackageReferenceNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	jsonResponse(ctx, http.StatusOK, &revisionInfo{Revision: revision.Value, Time: revision.CreatedUnix.AsLocalTime()})
}

// ListRecipeRevisionFiles gets a list of all recipe revision files
func ListRecipeRevisionFiles(ctx *context.Context) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	listRevisionFiles(ctx, rref.AsKey())
}

// ListPackageRevisionFiles gets a list of all package revision files
func ListPackageRevisionFiles(ctx *context.Context) {
	pref := ctx.Data[packageReferenceKey].(*conan_module.PackageReference)

	listRevisionFiles(ctx, pref.AsKey())
}

func listRevisionFiles(ctx *context.Context, fileKey string) {
	rref := ctx.Data[recipeReferenceKey].(*conan_module.RecipeReference)

	pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeConan, rref.Name, rref.Version)
	if err != nil {
		if err == packages_model.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
		} else {
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	pfs, _, err := packages_model.SearchFiles(ctx, &packages_model.PackageFileSearchOptions{
		VersionID:    pv.ID,
		CompositeKey: fileKey,
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pfs) == 0 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	files := make(map[string]any)
	for _, pf := range pfs {
		files[pf.Name] = nil
	}

	type FileList struct {
		Files map[string]any `json:"files"`
	}

	jsonResponse(ctx, http.StatusOK, &FileList{
		Files: files,
	})
}

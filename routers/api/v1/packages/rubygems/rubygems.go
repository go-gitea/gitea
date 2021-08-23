// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package rubygems

import (
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	rubygems_module "code.gitea.io/gitea/modules/packages/rubygems"
	"code.gitea.io/gitea/modules/util/filebuffer"

	package_service "code.gitea.io/gitea/services/packages"
)

// EnumeratePackages serves the package list
func EnumeratePackages(ctx *context.APIContext) {
	packages, err := models.GetPackagesByRepositoryAndType(ctx.Repo.Repository.ID, models.PackageRubyGems)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	enumeratePackages(ctx, "specs.4.8", packages)
}

// EnumeratePackagesLatest serves the list of the lastest version of every package
func EnumeratePackagesLatest(ctx *context.APIContext) {
	packages, _, err := models.GetLatestPackagesGrouped(&models.PackageSearchOptions{
		RepoID: ctx.Repo.Repository.ID,
		Type:   "rubygems",
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	enumeratePackages(ctx, "latest_specs.4.8", packages)
}

// EnumeratePackagesPreRelease is not supported and serves an empty list
func EnumeratePackagesPreRelease(ctx *context.APIContext) {
	enumeratePackages(ctx, "prerelease_specs.4.8", []*models.Package{})
}

func enumeratePackages(ctx *context.APIContext, filename string, packages []*models.Package) {
	rubygemsPackages, err := intializePackages(packages)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	specs := make([]interface{}, 0, len(rubygemsPackages))
	for _, p := range rubygemsPackages {
		specs = append(specs, []interface{}{
			p.Name,
			&rubygems_module.RubyUserMarshal{
				Name:  "Gem::Version",
				Value: []string{p.Version},
			},
			p.Metadata.Platform,
		})
	}

	ctx.SetServeHeaders(filename + ".gz")

	zw := gzip.NewWriter(ctx.Resp)
	defer zw.Close()

	zw.Name = filename

	if err := rubygems_module.NewMarshalEncoder(zw).Encode(specs); err != nil {
		ctx.ServerError("Download file failed", err)
	}
}

// ServePackageSpecification serves the compressed Gemspec file of a package
func ServePackageSpecification(ctx *context.APIContext) {
	filename := ctx.Params("filename")

	if !strings.HasSuffix(filename, ".gemspec.rz") {
		ctx.Error(http.StatusBadRequest, "", nil)
		return
	}

	packages, err := models.GetPackagesByFilename(ctx.Repo.Repository.ID, models.PackageRubyGems, filename[:len(filename)-10]+"gem")
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	if len(packages) != 1 {
		ctx.Error(http.StatusNotFound, "", nil)
		return
	}

	p, err := intializePackage(packages[0])
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	ctx.SetServeHeaders(filename)

	zw := zlib.NewWriter(ctx.Resp)
	defer zw.Close()

	spec := p.AsSpecification()

	if err := rubygems_module.NewMarshalEncoder(zw).Encode(spec); err != nil {
		ctx.ServerError("Download file failed", err)
	}
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.APIContext) {
	filename := ctx.Params("filename")

	packages, err := models.GetPackagesByFilename(ctx.Repo.Repository.ID, models.PackageRubyGems, filename)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	if len(packages) != 1 {
		ctx.Error(http.StatusNotFound, "", nil)
		return
	}

	s, pf, err := package_service.GetPackageFileStream(packages[0], filename)
	if err != nil {
		if err == models.ErrPackageFileNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackageFile adds a file to the package. If the package does not exist, it gets created.
func UploadPackageFile(ctx *context.APIContext) {
	upload, close, err := ctx.UploadStream()
	if err != nil {
		ctx.Error(http.StatusBadRequest, "", err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := filebuffer.CreateFromReader(upload, 32*1024*1024)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	defer buf.Close()

	meta, err := rubygems_module.ParsePackageMetaData(buf)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}
	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	p, err := package_service.CreatePackage(
		ctx.User,
		ctx.Repo.Repository,
		models.PackageRubyGems,
		meta.Name,
		meta.Version,
		meta,
		false,
	)
	if err != nil {
		if err == models.ErrDuplicatePackage {
			ctx.Error(http.StatusBadRequest, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	var filename string
	if len(meta.Platform) == 0 || meta.Platform == "ruby" {
		filename = strings.ToLower(fmt.Sprintf("%s-%s.gem", meta.Name, meta.Version))
	} else {
		filename = strings.ToLower(fmt.Sprintf("%s-%s-%s.gem", meta.Name, meta.Version, meta.Platform))
	}
	_, err = package_service.AddFileToPackage(p, filename, buf.Size(), buf)
	if err != nil {
		if err := models.DeletePackageByID(p.ID); err != nil {
			log.Error("Error deleting package by id: %v", err)
		}
		ctx.Error(http.StatusInternalServerError, "", err)
		return
	}

	ctx.PlainText(http.StatusCreated, nil)
}

// DeletePackage deletes a package
func DeletePackage(ctx *context.APIContext) {
	packageName := ctx.FormString("gem_name")
	packageVersion := ctx.FormString("version")

	err := package_service.DeletePackageByNameAndVersion(ctx.User, ctx.Repo.Repository, models.PackageRubyGems, packageName, packageVersion)
	if err != nil {
		if err == models.ErrPackageNotExist {
			ctx.Error(http.StatusNotFound, "", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "", "")
	}
}

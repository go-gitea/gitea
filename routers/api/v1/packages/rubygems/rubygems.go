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

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	//"code.gitea.io/gitea/modules/log"
	package_module "code.gitea.io/gitea/modules/packages"
	rubygems_module "code.gitea.io/gitea/modules/packages/rubygems"
	package_router "code.gitea.io/gitea/routers/api/v1/packages"
	packages_service "code.gitea.io/gitea/services/packages"
)

func apiError(ctx *context.APIContext, status int, obj interface{}) {
	package_router.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, []byte(message))
	})
}

// EnumeratePackages serves the package list
func EnumeratePackages(ctx *context.APIContext) {
	packages, err := packages.GetVersionsByPackageType(ctx.Repo.Repository.ID, packages.TypeRubyGems)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	enumeratePackages(ctx, "specs.4.8", packages)
}

// EnumeratePackagesLatest serves the list of the lastest version of every package
func EnumeratePackagesLatest(ctx *context.APIContext) {
	pvs, _, err := packages.SearchLatestVersions(&packages.PackageSearchOptions{
		RepoID: ctx.Repo.Repository.ID,
		Type:   "rubygems",
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	enumeratePackages(ctx, "latest_specs.4.8", pvs)
}

// EnumeratePackagesPreRelease is not supported and serves an empty list
func EnumeratePackagesPreRelease(ctx *context.APIContext) {
	enumeratePackages(ctx, "prerelease_specs.4.8", []*packages.PackageVersion{})
}

func enumeratePackages(ctx *context.APIContext, filename string, pvs []*packages.PackageVersion) {
	pds, err := packages.GetPackageDescriptors(pvs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	specs := make([]interface{}, 0, len(pds))
	for _, p := range pds {
		specs = append(specs, []interface{}{
			p.Package.Name,
			&rubygems_module.RubyUserMarshal{
				Name:  "Gem::Version",
				Value: []string{p.Version.Version},
			},
			p.Metadata.(*rubygems_module.Metadata).Platform,
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
		apiError(ctx, http.StatusNotImplemented, nil)
		return
	}

	pvs, err := packages.GetVersionsByFilename(ctx.Repo.Repository.ID, packages.TypeRubyGems, filename[:len(filename)-10]+"gem")
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pvs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	pd, err := packages.GetPackageDescriptor(pvs[0])
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.SetServeHeaders(filename)

	zw := zlib.NewWriter(ctx.Resp)
	defer zw.Close()

	metadata := pd.Metadata.(*rubygems_module.Metadata)

	// create a Ruby Gem::Specification object
	spec := &rubygems_module.RubyUserDef{
		Name: "Gem::Specification",
		Value: []interface{}{
			"3.2.3", // @rubygems_version
			4,       // @specification_version,
			pd.Package.Name,
			&rubygems_module.RubyUserMarshal{
				Name:  "Gem::Version",
				Value: []string{pd.Version.Version},
			},
			nil,               // date
			metadata.Summary,  // @summary
			nil,               // @required_ruby_version
			nil,               // @required_rubygems_version
			metadata.Platform, // @original_platform
			[]interface{}{},   // @dependencies
			nil,               // rubyforge_project
			"",                // @email
			metadata.Authors,
			metadata.Description,
			metadata.ProjectURL,
			true,              // has_rdoc
			metadata.Platform, // @new_platform
			nil,
			metadata.Licenses,
		},
	}

	if err := rubygems_module.NewMarshalEncoder(zw).Encode(spec); err != nil {
		ctx.ServerError("Download file failed", err)
	}
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.APIContext) {
	filename := ctx.Params("filename")

	pvs, err := packages.GetVersionsByFilename(ctx.Repo.Repository.ID, packages.TypeRubyGems, filename)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pvs) != 1 {
		apiError(ctx, http.StatusNotFound, nil)
		return
	}

	s, pf, err := packages_service.GetPackageFileStream(pvs[0], filename)
	if err != nil {
		if err == packages.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackageFile adds a file to the package. If the package does not exist, it gets created.
func UploadPackageFile(ctx *context.APIContext) {
	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := package_module.CreateHashedBufferFromReader(upload, 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	rp, err := rubygems_module.ParsePackageMetaData(buf)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if _, err := buf.Seek(0, io.SeekStart); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	var filename string
	if rp.Metadata.Platform == "" || rp.Metadata.Platform == "ruby" {
		filename = strings.ToLower(fmt.Sprintf("%s-%s.gem", rp.Name, rp.Version))
	} else {
		filename = strings.ToLower(fmt.Sprintf("%s-%s-%s.gem", rp.Name, rp.Version, rp.Metadata.Platform))
	}

	_, _, err = packages_service.CreatePackageAndAddFile(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Repository:  ctx.Repo.Repository,
				PackageType: packages.TypeRubyGems,
				Name:        rp.Name,
				Version:     rp.Version,
			},
			SemverCompatible: true,
			Creator:          ctx.User,
			Metadata:         rp.Metadata,
		},
		&packages_service.PackageFileInfo{
			Filename: filename,
			Data:     buf,
			IsLead:   true,
		},
	)
	if err != nil {
		if err == packages.ErrDuplicatePackageVersion {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusCreated, nil)
}

// DeletePackage deletes a package
func DeletePackage(ctx *context.APIContext) {
	// Go populates the form only for POST, PUT and PATCH requests
	if err := ctx.Req.ParseMultipartForm(32 << 20); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	packageName := ctx.FormString("gem_name")
	packageVersion := ctx.FormString("version")

	err := packages_service.DeletePackageVersionByNameAndVersion(
		ctx.User,
		&packages_service.PackageInfo{
			Repository:  ctx.Repo.Repository,
			PackageType: packages.TypeRubyGems,
			Name:        packageName,
			Version:     packageVersion,
		},
	)
	if err != nil {
		if err == packages.ErrPackageNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
	}
}

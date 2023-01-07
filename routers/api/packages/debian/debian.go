// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"
)

var (
	namePattern    = regexp.MustCompile(`\A[a-z0-9][a-z0-9\+\-\.]+\z`)
	versionPattern = regexp.MustCompile(`\A([0-9]:)?[a-zA-Z0-9\.\+\~]+(-[a-zA-Z0-9\.\+\~])?\z`) // TODO: hypens should be allowed if revision is present
	archPattern    = regexp.MustCompile(`\A[a-z0-9\-]+\z`)
)

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

func GetPackage(ctx *context.Context) {
	// Need to parse filename bc of how it's routed
	filename := ctx.Params("filename")
	log.Info("Filename: %s", filename)

	splitter := regexp.MustCompile(`^([^_]+)_([^_]+)_([^.]+).deb$`)
	matches := splitter.FindStringSubmatch(filename)
	if matches == nil {
		apiError(ctx, http.StatusBadRequest, "Invalid filename")
		return
	}
	packageName := matches[1]
	packageVersion := matches[2]

	s, pf, err := packages_service.GetFileStreamByPackageNameAndVersion(
		ctx,
		&packages_service.PackageInfo{
			Owner:       ctx.Package.Owner,
			PackageType: packages_model.TypeDebian,
			Name:        packageName,
			Version:     packageVersion,
		},
		&packages_service.PackageFileInfo{
			Filename: filename,
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

func PutPackage(ctx *context.Context) {
	packageName := ctx.Params("packagename")

	if !namePattern.MatchString(packageName) {
		apiError(ctx, http.StatusBadRequest, errors.New("Invalid package name"))
		return
	}

	packageVersion := ctx.Params("packageversion")
	if packageVersion != strings.TrimSpace(packageVersion) {
		apiError(ctx, http.StatusBadRequest, errors.New("Invalid package version"))
		return
	}

	packageArch := ctx.Params("arch")
	// TODO Check arch

	upload, close, err := ctx.UploadStream()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if close {
		defer upload.Close()
	}

	buf, err := packages_module.CreateHashedBufferFromReader(upload, 32*1024*1024)
	if err != nil {
		log.Error("Error creating hashed buffer: %v", err)
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	filename := fmt.Sprintf("%s_%s_%s.deb", packageName, packageVersion, packageArch)
	_, _, err = packages_service.CreatePackageOrAddFileToExisting(
		&packages_service.PackageCreationInfo{
			PackageInfo: packages_service.PackageInfo{
				Owner:       ctx.Package.Owner,
				PackageType: packages_model.TypeDebian,
				Name:        packageName,
				Version:     packageVersion,
			},
			Creator: ctx.Doer,
		},
		&packages_service.PackageFileCreationInfo{
			PackageFileInfo: packages_service.PackageFileInfo{
				Filename: filename,
			},
			Creator: ctx.Doer,
			Data:    buf,
			IsLead:  true,
		},
	)
	if err != nil {
		switch err {
		case packages_model.ErrDuplicatePackageFile:
			apiError(ctx, http.StatusConflict, err)
		case packages_service.ErrQuotaTotalCount, packages_service.ErrQuotaTypeSize, packages_service.ErrQuotaTotalSize:
			apiError(ctx, http.StatusForbidden, err)
		default:
			apiError(ctx, http.StatusInternalServerError, err)
		}
		return
	}

	DebianRepoUpdate(ctx, packageArch)
	ctx.Status(http.StatusCreated)
}

func DeletePackage(ctx *context.Context) {
	packageName := ctx.Params("packagename")
	packageVersion := ctx.Params("packageversion")
	packageArch := ctx.Params("arch")
	filename := fmt.Sprintf("%s_%s_%s.deb", packageName, packageVersion, packageArch)

	pv, pf, err := func() (*packages_model.PackageVersion, *packages_model.PackageFile, error) {
		pv, err := packages_model.GetVersionByNameAndVersion(ctx, ctx.Package.Owner.ID, packages_model.TypeDebian, packageName, packageVersion)
		if err != nil {
			return nil, nil, err
		}

		pf, err := packages_model.GetFileForVersionByName(ctx, pv.ID, filename, packages_model.EmptyFileKey)
		if err != nil {
			return nil, nil, err
		}

		return pv, pf, nil
	}()
	if err != nil {
		if err == packages_model.ErrPackageNotExist || err == packages_model.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	pfs, err := packages_model.GetFilesByVersionID(ctx, pv.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if len(pfs) == 1 {
		if err := packages_service.RemovePackageVersion(ctx.Doer, pv); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	} else {
		if err := packages_service.DeletePackageFile(ctx, pf); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	DebianRepoUpdate(ctx, packageArch)
	ctx.Status(http.StatusNoContent)
}

func GetDebianFileDescriptors(ctx *context.Context) ([]*packages_model.PackageFileDescriptor, error) {
	pvs, err := packages_model.GetVersionsByPackageType(ctx, ctx.Package.Owner.ID, packages_model.TypeDebian)
	if err != nil {
		return nil, err
	}

	pds, err := packages_model.GetPackageDescriptors(ctx, pvs)
	if err != nil {
		return nil, err
	}

	files := make([]*packages_model.PackageFileDescriptor, 0)
	for _, pd := range pds {
		files = append(files, pd.Files...)
	}

	return files, nil
}

func GetDebianFilesByArch(ctx *context.Context) (map[string][]*packages_model.PackageFileDescriptor, error) {
	pfds, err := GetDebianFileDescriptors(ctx)
	if err != nil {
		return nil, err
	}

	splitter := regexp.MustCompile(`^([^_]+)_([^_]+)_([^.]+).deb$`)

	files := make(map[string][]*packages_model.PackageFileDescriptor)

	for _, pfd := range pfds {
		filename := pfd.File.Name
		matches := splitter.FindStringSubmatch(filename)
		if matches == nil || len(matches) != 4 {
			log.Error("Found invalid filename: %s", filename)
			return nil, errors.New("Found invalid filename")
		}

		arch := matches[3]
		files[arch] = append(files[arch], pfd)
	}

	return files, nil
}

func GetArchIndex(ctx *context.Context) {
	ctx.Data["IndexFiles"] = map[string]string{
		"../":         "../",
		"Packages":    "Packages",
		"Packages.gz": "Packages.gz",
		"Release":     "Release",
	}

	// This does mean that "amd64" and "binary-amd64" can both be used
	// Don't think that's an issue (?)
	arch := ctx.Params("packagearch")
	if len(arch) > 7 && arch[:7] == "binary-" {
		arch = arch[7:]
	}

	archs, err := GetDebianFilesByArch(ctx)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, exists := archs[arch]
	if !exists {
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.Data["IndexPath"] = ctx.Req.URL.Path
	ctx.HTML(http.StatusOK, "api/packages/debian/index")
}

func GetIndex(ctx *context.Context) {
	basePath := "/api/packages/" + ctx.Params("username") + "/debian"

	relPath, err := filepath.Rel(basePath, ctx.Req.URL.Path)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		log.Error("Path '%s' is not inside '%s'?", ctx.Req.URL.Path, basePath)
		return
	}

	log.Info("RelPath: %s", relPath)

	switch relPath {
	case ".":
		ctx.Data["IndexFiles"] = map[string]string{
			"../":        "./",
			"dists/":     "dists/",
			"pool/":      "pool/",
			"debian.key": "debian.key",
		}
	case "pool":
		files := map[string]string{
			"../": "../",
		}

		// Add all the Debian package files
		pfds, err := GetDebianFileDescriptors(ctx)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		for _, pfd := range pfds {
			files[pfd.File.Name] = pfd.File.Name
		}

		ctx.Data["IndexFiles"] = files
	case "dists":
		ctx.Data["IndexFiles"] = map[string]string{
			"../":    "../",
			"gitea/": "gitea/",
		}
	case "dists/gitea":
		ctx.Data["IndexFiles"] = map[string]string{
			"../":         "../",
			"main/":       "main/",
			"InRelease":   "InRelease",
			"Release":     "Release",
			"Release.gpg": "Release.gpg",
		}
	case "dists/gitea/main":
		files := map[string]string{
			"../": "../",
		}

		// Add directory for each arch
		archs, err := GetDebianFilesByArch(ctx)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		for a := range archs {
			var name string
			switch a {
			case "source":
				name = a + "/"
			default:
				name = "binary-" + a + "/"
			}
			files[name] = name
		}

		ctx.Data["IndexFiles"] = files
	}
	ctx.Data["IndexPath"] = ctx.Req.URL.Path
	ctx.HTML(http.StatusOK, "api/packages/debian/index")
}

func DebianRepoUpdate(ctx *context.Context, arch string) {
	if err := CreatePackagesGz(ctx, arch); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if err := CreateRelease(ctx); err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
}

func GetArchRelease(ctx *context.Context) {
	data := fmt.Sprintf("Component: main\nArchitecture: %s\n", ctx.Params("packagearch"))
	ctx.PlainText(http.StatusOK, data)
}

func GetPackages(ctx *context.Context) {
	basePath := filepath.Join(setting.AppDataPath, "debian_repo")
	archPath := filepath.Join(basePath, ctx.Package.Owner.Name, ctx.Params("packagearch"))
	packagesPath := filepath.Join(archPath, "Packages")
	f, err := os.Open(packagesPath)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
	}
	ctx.ServeContent(f, &context.ServeHeaderOptions{
		Filename: "Packages",
	})
}

func GetPackagesGZ(ctx *context.Context) {
	basePath := filepath.Join(setting.AppDataPath, "debian_repo")
	archPath := filepath.Join(basePath, ctx.Package.Owner.Name, ctx.Params("packagearch"))
	packagesGzPath := filepath.Join(archPath, "Packages.gz")
	f, err := os.Open(packagesGzPath)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
	}
	ctx.ServeContent(f, &context.ServeHeaderOptions{
		Filename: "Packages.gz",
	})
}

func GetRelease(ctx *context.Context) {
	path := filepath.Join(setting.AppDataPath, "debian_repo", ctx.Package.Owner.Name, "Release")
	f, err := os.Open(path)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
	}
	ctx.ServeContent(f, &context.ServeHeaderOptions{
		Filename: "Release",
	})
}

func GetReleaseGPG(ctx *context.Context) {
	path := filepath.Join(setting.AppDataPath, "debian_repo", ctx.Package.Owner.Name, "Release.gpg")
	f, err := os.Open(path)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
	}
	ctx.ServeContent(f, &context.ServeHeaderOptions{
		Filename: "Release.gpg",
	})
}

func GetInRelease(ctx *context.Context) {
	path := filepath.Join(setting.AppDataPath, "debian_repo", ctx.Package.Owner.Name, "InRelease")
	f, err := os.Open(path)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
	}
	ctx.ServeContent(f, &context.ServeHeaderOptions{
		Filename: "InRelease",
	})
}

func GetPublicKey(ctx *context.Context) {
	path := filepath.Join(setting.AppDataPath, "debian_public.gpg")
	f, err := os.Open(path)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
	}
	ctx.ServeContent(f, &context.ServeHeaderOptions{
		Filename: "debian.key",
	})
}

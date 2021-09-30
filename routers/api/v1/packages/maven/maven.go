// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package maven

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	maven_module "code.gitea.io/gitea/modules/packages/maven"
	"code.gitea.io/gitea/modules/util/filebuffer"
	package_router "code.gitea.io/gitea/routers/api/v1/packages"
	package_service "code.gitea.io/gitea/services/packages"
)

const (
	mavenMetadataFile = "maven-metadata.xml"
	extensionMD5      = ".md5"
	extensionSHA1     = ".sha1"
	extensionSHA256   = ".sha256"
	extensionSHA512   = ".sha512"
)

var (
	errInvalidParameters = errors.New("request parameters are invalid")
	illegalCharacters    = regexp.MustCompile(`[\\/:"<>|?\*]`)
)

func apiError(ctx *context.APIContext, status int, obj interface{}) {
	package_router.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, []byte(message))
	})
}

// DownloadPackageFile serves the content of a package
func DownloadPackageFile(ctx *context.APIContext) {
	params, err := extractPathParameters(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	if params.IsMeta && params.Version == "" {
		serveMavenMetadata(ctx, params)
	} else {
		servePackageFile(ctx, params)
	}
}

func serveMavenMetadata(ctx *context.APIContext, params parameters) {
	// /com/foo/project/maven-metadata.xml[.md5/.sha1/.sha256/.sha512]

	packageName := params.GroupID + "-" + params.ArtifactID
	pkgs, err := packages.GetPackagesByName(ctx.Repo.Repository.ID, packages.TypeMaven, packageName)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	if len(pkgs) == 0 {
		apiError(ctx, http.StatusNotFound, packages.ErrPackageNotExist)
		return
	}

	mavenPackages, err := intializePackages(pkgs)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	xmlMetadata, err := xml.Marshal(createMetadataResponse(mavenPackages))
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	xmlMetadataWithHeader := append([]byte(xml.Header), xmlMetadata...)

	ext := strings.ToLower(filepath.Ext(params.Filename))
	if isChecksumExtension(ext) {
		var hash []byte
		switch ext {
		case extensionMD5:
			tmp := md5.Sum(xmlMetadataWithHeader)
			hash = tmp[:]
		case extensionSHA1:
			tmp := sha1.Sum(xmlMetadataWithHeader)
			hash = tmp[:]
		case extensionSHA256:
			tmp := sha256.Sum256(xmlMetadataWithHeader)
			hash = tmp[:]
		case extensionSHA512:
			tmp := sha512.Sum512(xmlMetadataWithHeader)
			hash = tmp[:]
		}
		ctx.PlainText(http.StatusOK, []byte(fmt.Sprintf("%x", hash)))
		return
	}

	ctx.PlainText(http.StatusOK, xmlMetadataWithHeader)
}

func servePackageFile(ctx *context.APIContext, params parameters) {
	packageName := params.GroupID + "-" + params.ArtifactID

	p, err := packages.GetPackageByNameAndVersion(ctx.Repo.Repository.ID, packages.TypeMaven, packageName, params.Version)
	if err == packages.ErrPackageNotExist {
		apiError(ctx, http.StatusNotFound, err)
		return
	}

	filename := params.Filename

	ext := strings.ToLower(filepath.Ext(filename))
	if isChecksumExtension(ext) {
		filename = filename[:len(filename)-len(ext)]
	}

	pf, err := p.GetFileByName(filename)
	if err != nil {
		if err == packages.ErrPackageFileNotExist {
			apiError(ctx, http.StatusNotFound, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	if isChecksumExtension(ext) {
		var hash string
		switch ext {
		case extensionMD5:
			hash = pf.HashMD5
		case extensionSHA1:
			hash = pf.HashSHA1
		case extensionSHA256:
			hash = pf.HashSHA256
		case extensionSHA512:
			hash = pf.HashSHA512
		}
		ctx.PlainText(http.StatusOK, []byte(hash))
		return
	}

	s, err := packages_module.NewContentStore().Get(p.ID, pf.ID)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
	}
	defer s.Close()

	ctx.ServeStream(s, pf.Name)
}

// UploadPackageFile adds a file to the package. If the package does not exist, it gets created.
func UploadPackageFile(ctx *context.APIContext) {
	params, err := extractPathParameters(ctx)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	log.Trace("Parameters: %+v", params)

	// Ignore the package index /<name>/maven-metadata.xml
	if params.IsMeta && params.Version == "" {
		ctx.PlainText(http.StatusOK, nil)
		return
	}

	packageName := params.GroupID + "-" + params.ArtifactID

	p, err := package_service.CreatePackage(
		ctx.User,
		ctx.Repo.Repository,
		packages.TypeMaven,
		packageName,
		params.Version,
		&maven_module.Metadata{
			GroupID:    params.GroupID,
			ArtifactID: params.ArtifactID,
		},
		true,
	)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	buf, err := filebuffer.CreateFromReader(ctx.Req.Body, 32*1024*1024)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer buf.Close()

	ext := filepath.Ext(params.Filename)

	// Do not upload checksum files but compare the hashes.
	if isChecksumExtension(ext) {
		pf, err := p.GetFileByName(params.Filename[:len(params.Filename)-len(ext)])
		if err != nil {
			if err == packages.ErrPackageFileNotExist {
				apiError(ctx, http.StatusNotFound, err)
				return
			}
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		hash, err := ioutil.ReadAll(buf)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		if (ext == extensionMD5 && pf.HashMD5 != string(hash)) ||
			(ext == extensionSHA1 && pf.HashSHA1 != string(hash)) ||
			(ext == extensionSHA256 && pf.HashSHA256 != string(hash)) ||
			(ext == extensionSHA512 && pf.HashSHA512 != string(hash)) {
			apiError(ctx, http.StatusBadRequest, "hash mismatch")
			return
		}

		ctx.PlainText(http.StatusOK, nil)
		return
	}

	// If it's the package pom file extract the metadata
	if ext == ".pom" {
		metadata, err := maven_module.ParsePackageMetaData(buf)
		if err != nil {
			log.Error("Error parsing package metadata: %v", err)
		}
		if metadata != nil {
			raw, err := json.Marshal(metadata)
			if err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
			p.MetadataRaw = string(raw)
			if err := packages.UpdatePackage(p); err != nil {
				apiError(ctx, http.StatusInternalServerError, err)
				return
			}
		}
		if _, err := buf.Seek(0, io.SeekStart); err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	_, err = package_service.AddFileToPackage(p, params.Filename, buf.Size(), buf)
	if err != nil {
		if err == packages.ErrDuplicatePackageFile {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.PlainText(http.StatusCreated, nil)
}

func isChecksumExtension(ext string) bool {
	return ext == extensionMD5 || ext == extensionSHA1 || ext == extensionSHA256 || ext == extensionSHA512
}

type parameters struct {
	GroupID    string
	ArtifactID string
	Version    string
	Filename   string
	IsMeta     bool
}

func extractPathParameters(ctx *context.APIContext) (parameters, error) {
	parts := strings.Split(ctx.Params("*"), "/")

	p := parameters{
		Filename: parts[len(parts)-1],
	}

	p.IsMeta = p.Filename == mavenMetadataFile ||
		p.Filename == mavenMetadataFile+extensionMD5 ||
		p.Filename == mavenMetadataFile+extensionSHA1 ||
		p.Filename == mavenMetadataFile+extensionSHA256 ||
		p.Filename == mavenMetadataFile+extensionSHA512

	parts = parts[:len(parts)-1]
	if len(parts) == 0 {
		return p, errInvalidParameters
	}

	p.Version = parts[len(parts)-1]
	if p.IsMeta && !strings.HasSuffix(p.Version, "-SNAPSHOT") {
		p.Version = ""
	} else {
		parts = parts[:len(parts)-1]
	}

	if illegalCharacters.MatchString(p.Version) {
		return p, errInvalidParameters
	}

	if len(parts) < 2 {
		return p, errInvalidParameters
	}

	p.ArtifactID = parts[len(parts)-1]
	p.GroupID = strings.Join(parts[:len(parts)-1], ".")

	if illegalCharacters.MatchString(p.GroupID) || illegalCharacters.MatchString(p.ArtifactID) {
		return p, errInvalidParameters
	}

	return p, nil
}

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	packages_module "code.gitea.io/gitea/modules/packages"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/routers/api/packages/helper"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/google/uuid"
)

// Push new package to arch package registry.
func Push(ctx *context.Context) {
	// Creating connector that will help with keys/blobs.
	connector := Connector{ctx: ctx}

	// Getting some information related to package from headers.
	filename := ctx.Req.Header.Get("filename")
	email := ctx.Req.Header.Get("email")
	sign := ctx.Req.Header.Get("sign")
	owner := ctx.Req.Header.Get("owner")
	distro := ctx.Req.Header.Get("distro")

	// Decoding package signature.
	sigdata, err := hex.DecodeString(sign)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	pgpsig := crypto.NewPGPSignature(sigdata)

	// Validating that user is allowed to push to specified namespace.
	err = connector.ValidateNamespace(owner, email)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	// Getting GPG keys related to specific user. After keys have been recieved,
	// this function will find one key related to email provided in request.
	armoredKeys, err := connector.GetValidKeys(email)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}
	var matchedKeyring *crypto.KeyRing
	for _, armor := range armoredKeys {
		pgpkey, err := crypto.NewKeyFromArmored(armor)
		if err != nil {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		keyring, err := crypto.NewKeyRing(pgpkey)
		if err != nil {
			apiError(ctx, http.StatusBadRequest, err)
			return
		}
		for _, idnt := range keyring.GetIdentities() {
			if idnt.Email == email {
				matchedKeyring = keyring
				break
			}
		}
		if matchedKeyring != nil {
			break
		}
	}
	if matchedKeyring == nil {
		msg := "GPG key related to " + email + " not found"
		apiError(ctx, http.StatusBadRequest, msg)
		return
	}

	// Read package to memory and create plain GPG message to validate signature.
	pkgdata, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer ctx.Req.Body.Close()

	pgpmes := crypto.NewPlainMessage(pkgdata)

	// Validate package signature with user's GPG key related to his email.
	err = matchedKeyring.VerifyDetached(pgpmes, pgpsig, crypto.GetUnixTime())
	if err != nil {
		apiError(ctx, http.StatusUnauthorized, "unable to validate package signature")
		return
	}

	// Create temporary directory for arch database operations.
	tmpdir := path.Join(setting.Repository.Upload.TempPath, uuid.New().String())
	err = os.MkdirAll(tmpdir, os.ModePerm)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, "unable to create tmp path")
		return
	}
	defer os.RemoveAll(tmpdir)

	// Parse metadata contained in arch package archive.
	md, err := arch_module.EjectMetadata(filename, setting.Domain, pkgdata)
	if err != nil {
		apiError(ctx, http.StatusBadRequest, err)
		return
	}

	// Arch database related filenames, pathes and folders.
	dbname := Join(owner, distro, setting.Domain, "db.tar.gz")
	dbpath := path.Join(tmpdir, dbname)
	dbfolder := path.Join(tmpdir, dbname) + ".folder"
	dbsymlink := strings.TrimSuffix(dbname, ".tar.gz")
	dbsymlinkpath := path.Join(tmpdir, dbsymlink)

	// Get existing arch package database, related to specific userspace from
	// file storage, and save it on disk, then unpack it's contents to related
	// folder. If database is not found in storage, create empty directory to
	// store package related information.
	dbdata, err := connector.Get(dbname)
	if err == nil {
		err = os.WriteFile(dbpath, dbdata, os.ModePerm)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		err = arch_module.UnpackDb(dbpath, dbfolder)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}
	if err != nil {
		err = os.MkdirAll(dbfolder, os.ModePerm)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	// Update database folder with metadata for new package.
	err = md.PutToDb(dbfolder, os.ModePerm)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Create database archive and related symlink.
	err = arch_module.PackDb(dbfolder, dbpath)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Save namespace related arch repository database.
	f, err := os.Open(dbpath)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()
	dbfi, err := f.Stat()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	err = connector.Save(dbname, f, dbfi.Size())
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Save namespace related arch repository db archive.
	f, err = os.Open(dbsymlinkpath)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()
	dbarchivefi, err := f.Stat()
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	err = connector.Save(dbsymlink, f, dbarchivefi.Size())
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Create package in database.
	pkg, err := packages_model.TryInsertPackage(ctx, &packages_model.Package{
		OwnerID:   connector.user.ID,
		Type:      packages_model.TypeArch,
		Name:      md.Name,
		LowerName: strings.ToLower(md.Name),
	})
	if errors.Is(err, packages_model.ErrDuplicatePackage) {
		pkg, err = packages_model.GetPackageByName(
			ctx, connector.user.ID,
			packages_model.TypeArch, md.Name,
		)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Check if repository for package with provided owner exists.
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner, md.Name)
	if err == nil {
		err = packages_model.SetRepositoryLink(ctx, pkg.ID, repo.ID)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	// Create new package version in database.
	rawjsonmetadata, err := json.Marshal(&md)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ver, err := packages_model.GetOrInsertVersion(ctx, &packages_model.PackageVersion{
		PackageID:    pkg.ID,
		CreatorID:    connector.user.ID,
		Version:      md.Version,
		LowerVersion: strings.ToLower(md.Version),
		CreatedUnix:  timeutil.TimeStampNow(),
		MetadataJSON: string(rawjsonmetadata),
	})
	if err != nil {
		if errors.Is(err, packages_model.ErrDuplicatePackageVersion) {
			apiError(ctx, http.StatusConflict, err)
			return
		}
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Create package blob and db file for package file.
	pkgreader := bytes.NewReader(pkgdata)
	fbuf, err := packages_module.CreateHashedBufferFromReader(pkgreader)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer fbuf.Close()

	filepb, ok, err := packages_model.GetOrInsertBlob(
		ctx, packages_service.NewPackageBlob(fbuf),
	)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, fmt.Errorf("%v %t", err, ok))
		return
	}
	err = connector.Save(filepb.HashSHA256, fbuf, filepb.Size)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, err = packages_model.TryInsertFile(ctx, &packages_model.PackageFile{
		VersionID:    ver.ID,
		BlobID:       filepb.ID,
		Name:         filename,
		LowerName:    strings.ToLower(filename),
		CompositeKey: distro + "-" + filename,
		IsLead:       true,
		CreatedUnix:  timeutil.TimeStampNow(),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	// Create package blob for package signature.
	sigreader := bytes.NewReader(sigdata)
	sbuf, err := packages_module.CreateHashedBufferFromReader(sigreader)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	defer fbuf.Close()

	sigpb, ok, err := packages_model.GetOrInsertBlob(
		ctx, packages_service.NewPackageBlob(sbuf),
	)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, fmt.Errorf("%v %t", err, ok))
		return
	}
	err = connector.Save(sigpb.HashSHA256, sbuf, sigpb.Size)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, err = packages_model.TryInsertFile(ctx, &packages_model.PackageFile{
		VersionID:    ver.ID,
		BlobID:       sigpb.ID,
		Name:         filename + ".sig",
		LowerName:    strings.ToLower(filename + ".sig"),
		CompositeKey: distro + "-" + filename + ".sig",
		IsLead:       false,
		CreatedUnix:  timeutil.TimeStampNow(),
	})
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.Status(http.StatusOK)
}

// Get file from arch package registry.
func Get(ctx *context.Context) {
	filename := ctx.Params("file")
	owner := ctx.Params("owner")
	distro := ctx.Params("distro")
	// arch := ctx.Params("arch")

	cs := packages_module.NewContentStore()

	if strings.HasSuffix(filename, "tar.zst") ||
		strings.HasSuffix(filename, "zst.sig") {
		db := db.GetEngine(ctx)

		pkgfile := &packages_model.PackageFile{
			CompositeKey: distro + "-" + filename,
		}
		ok, err := db.Get(pkgfile)
		if err != nil || !ok {
			apiError(
				ctx, http.StatusInternalServerError,
				fmt.Errorf("%+v %t", err, ok),
			)
			return
		}

		blob, err := packages_model.GetBlobByID(ctx, pkgfile.BlobID)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		obj, err := cs.Get(packages_module.BlobHash256Key(blob.HashSHA256))
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		data, err := io.ReadAll(obj)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}

		_, err = ctx.Resp.Write(data)
		if err != nil {
			apiError(ctx, http.StatusInternalServerError, err)
			return
		}
		ctx.Resp.WriteHeader(http.StatusOK)

		return
	}
	obj, err := cs.Get(packages_module.BlobHash256Key(Join(owner, distro, filename)))
	if err != nil {
		apiError(ctx, http.StatusNotFound, err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}

	_, err = ctx.Resp.Write(data)
	if err != nil {
		apiError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.Resp.WriteHeader(http.StatusOK)
}

func apiError(ctx *context.Context, status int, obj interface{}) {
	helper.LogAndProcessError(ctx, status, obj, func(message string) {
		ctx.PlainText(status, message)
	})
}

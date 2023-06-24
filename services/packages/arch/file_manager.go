// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/models/db"
	pkg_mdl "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/setting"
	"github.com/google/uuid"
)

// Get data related to provided file name and distribution, and update download
// counter if actual package file is retrieved from database.
func LoadPackageFile(ctx *context.Context, distro, file string) ([]byte, error) {
	db := db.GetEngine(ctx)

	pkgfile := &pkg_mdl.PackageFile{CompositeKey: distro + "-" + file}

	ok, err := db.Get(pkgfile)
	if err != nil || !ok {
		return nil, fmt.Errorf("%+v %t", err, ok)
	}

	blob, err := pkg_mdl.GetBlobByID(ctx, pkgfile.BlobID)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(file, ".pkg.tar.zst") {
		err = pkg_mdl.IncrementDownloadCounter(ctx, pkgfile.VersionID)
		if err != nil {
			return nil, err
		}
	}

	cs := packages.NewContentStore()

	obj, err := cs.Get(packages.BlobHash256Key(blob.HashSHA256))
	if err != nil {
		return nil, err
	}

	return io.ReadAll(obj)
}

// Get data related to pacman database file or symlink.
func LoadPacmanDatabase(ctx *context.Context, owner, distro, architecture, file string) ([]byte, error) {
	cs := packages.NewContentStore()

	file = strings.TrimPrefix(file, owner+".")

	dbname := strings.TrimSuffix(arch.Join(owner, distro, architecture, file), ".tar.gz")

	obj, err := cs.Get(packages.BlobHash256Key(dbname))
	if err != nil {
		return nil, err
	}

	return io.ReadAll(obj)
}

// This function will update information about package in related pacman databases
// or create them if they do not exist.
func UpdatePacmanDatabases(ctx *context.Context, md *arch.Metadata, distro, owner string) error {
	// Create temporary directory for arch database operations.
	tmpdir := path.Join(setting.Repository.Upload.TempPath, uuid.New().String())
	err := os.MkdirAll(tmpdir, os.ModePerm)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	// If architecure is not specified or any, package will be automatically
	// saved to pacman databases with most popular architectures.
	var architectures = md.Arch
	if len(md.Arch) == 0 || md.Arch[0] == "any" {
		architectures = []string{
			"x86_64", "arm", "i686", "pentium4",
			"armv7h", "armv6h", "aarch64", "riscv64",
		}
	}

	cs := packages.NewContentStore()

	// Update pacman database files for each architecture.
	for _, architecture := range architectures {
		db := arch.Join(owner, distro, architecture, setting.Domain, "db")
		dbkey := packages.BlobHash256Key(db)

		o, err := cs.Get(dbkey)
		if err != nil {
			return err
		}

		data, err := io.ReadAll(o)
		if err != nil {
			return err
		}

		udata, err := arch.UpdatePacmanDbEntry(data, md)
		if err != nil {
			return err
		}

		err = cs.Save(dbkey, bytes.NewReader(udata), int64(len(udata)))
		if err != nil {
			return err
		}
	}

	return nil
}

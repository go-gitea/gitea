// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
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

// Get data related to provided file name and distribution.
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

	obj, err := cs.Get(packages.BlobHash256Key(arch.Join(owner, distro, architecture, file)))
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
	// saved to databases with most popular architectures.
	var architectures = md.Arch
	if len(md.Arch) == 0 || md.Arch[0] == "any" {
		architectures = []string{
			"x86_64", "arm", "i686", "pentium4",
			"armv7h", "armv6h", "aarch64", "riscv64",
		}
	}

	cs := packages.NewContentStore()

	for _, architecture := range architectures {
		var (
			db    = arch.Join(owner, distro, architecture, setting.Domain, "db.tar.gz")
			dbpth = path.Join(tmpdir, db)
			dbf   = path.Join(tmpdir, db) + ".folder"
			sbsl  = strings.TrimSuffix(db, ".tar.gz")
			slpth = path.Join(tmpdir, sbsl)
		)

		// Get existing pacman database, or create empty folder for it.
		dbdata, err := cs.GetStrBytes(db)
		if err == nil {
			err = os.WriteFile(dbpth, dbdata, os.ModePerm)
			if err != nil {
				return err
			}
			err = arch.UnpackDb(dbpth, dbf)
			if err != nil {
				return err
			}
		}
		if err != nil {
			err = os.MkdirAll(dbf, os.ModePerm)
			if err != nil {
				return err
			}
		}

		// Update database folder with metadata for new package.
		err = md.PutToDb(dbf, os.ModePerm)
		if err != nil {
			return err
		}

		// Create database archive and related symlink.
		err = arch.PackDb(dbf, dbpth)
		if err != nil {
			return err
		}

		// Save database file.
		f, err := os.Open(dbpth)
		if err != nil {
			return err
		}
		defer f.Close()
		dbfi, err := f.Stat()
		if err != nil {
			return err
		}
		err = cs.Save(packages.BlobHash256Key(db), f, dbfi.Size())
		if err != nil {
			return err
		}

		// Save database symlink file.
		f, err = os.Open(slpth)
		if err != nil {
			return err
		}
		defer f.Close()
		dbarchivefi, err := f.Stat()
		if err != nil {
			return err
		}
		err = cs.Save(packages.BlobHash256Key(sbsl), f, dbarchivefi.Size())
		if err != nil {
			return err
		}
	}
	return nil
}

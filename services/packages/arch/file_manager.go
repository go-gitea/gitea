// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/models/db"
	pkg_mdl "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/modules/setting"
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
	// If architecure is not specified or any, package will be automatically
	// saved to pacman databases with most popular architectures.
	if len(md.Arch) == 0 || md.Arch[0] == "any" {
		md.Arch = popularArchitectures()
	}

	cs := packages.NewContentStore()

	// Update pacman database files for each architecture.
	for _, architecture := range md.Arch {
		db := arch.Join(owner, distro, architecture, setting.Domain, "db")
		dbkey := packages.BlobHash256Key(db)

		var dbdata []byte

		dbobj, err := cs.Get(dbkey)
		if err == nil {
			dbdata, err = io.ReadAll(dbobj)
			if err != nil {
				return err
			}
		}

		newdata, err := arch.UpdatePacmanDbEntry(dbdata, md)
		if err != nil {
			return err
		}

		err = cs.Save(dbkey, bytes.NewReader(newdata), int64(len(newdata)))
		if err != nil {
			return err
		}
	}

	return nil
}

func RemoveDbEntry(ctx *context.Context, architectures []string, owner, distro, pkg, ver string) error {
	cs := packages.NewContentStore()

	// If architecures are not specified or any, package will be automatically
	// removed from pacman databases with most popular architectures.
	if len(architectures) == 0 || architectures[0] == "any" {
		architectures = popularArchitectures()
	}

	for _, architecture := range architectures {
		db := arch.Join(owner, distro, architecture, setting.Domain, "db")
		dbkey := packages.BlobHash256Key(db)

		var dbdata []byte

		dbobj, err := cs.Get(dbkey)
		if err != nil {
			return err
		}

		dbdata, err = io.ReadAll(dbobj)
		if err != nil {
			return err
		}

		newdata, err := arch.RemoveDbEntry(dbdata, pkg, ver)
		if err != nil {
			return err
		}

		err = cs.Save(dbkey, bytes.NewReader(newdata), int64(len(newdata)))
		if err != nil {
			return err
		}
	}
	return nil
}

func popularArchitectures() []string {
	return []string{
		"x86_64", "arm", "i686", "pentium4",
		"armv7h", "armv6h", "aarch64", "riscv64",
	}
}

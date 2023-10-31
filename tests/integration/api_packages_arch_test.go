// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"testing"
	"testing/fstest"
	"time"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/tests"

	"github.com/mholt/archiver/v3"
	"github.com/minio/sha256-simd"
	"github.com/stretchr/testify/assert"
)

func TestPackageArch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	var (
		gitV1x86_64 = BuildArchPackage(t, "git", "1-1", "x86_64")
		gitV1i686   = BuildArchPackage(t, "git", "1-1", "i686")
		iconsV1any  = BuildArchPackage(t, "icons", "1-1", "any")
		gitV2x86_64 = BuildArchPackage(t, "git", "2-1", "x86_64")
		iconsV2any  = BuildArchPackage(t, "icons", "2-1", "any")

		firstSign  = []byte{1, 2, 3, 4}
		secondSign = []byte{4, 3, 2, 1}

		V1x86_64database = BuildArchDatabase([]arch.Package{
			gitV1x86_64.pkg,
			iconsV1any.pkg,
		})
		V1i686database = BuildArchDatabase([]arch.Package{
			gitV1i686.pkg,
			iconsV1any.pkg,
		})

		V2x86_64database = BuildArchDatabase([]arch.Package{
			gitV2x86_64.pkg,
			iconsV2any.pkg,
		})
		V2i686database = BuildArchDatabase([]arch.Package{
			gitV1i686.pkg,
			iconsV2any.pkg,
		})

		user    = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		rootURL = fmt.Sprintf("/api/packages/%s/arch", user.Name)
	)

	t.Run("Version_1", func(t *testing.T) {
		t.Run("Push_git_x86_64", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT",
				path.Join(
					rootURL, "push", "git-1-1-x86_64.pkg.tar.zst",
					"archlinux", hex.EncodeToString(firstSign),
				),
				bytes.NewReader(gitV1x86_64.data),
			)

			req = AddBasicAuthHeader(req, user.Name)

			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Push_git_i686", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT",
				path.Join(
					rootURL, "push", "git-1-1-i686.pkg.tar.zst",
					"archlinux", hex.EncodeToString(secondSign),
				),
				bytes.NewReader(gitV1i686.data),
			)

			req = AddBasicAuthHeader(req, user.Name)

			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Push_icons_any", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT",
				path.Join(rootURL, "push", "icons-1-1-any.pkg.tar.zst", "archlinux"),
				bytes.NewReader(iconsV1any.data),
			)

			req = AddBasicAuthHeader(req, user.Name)

			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Get_git_x86_64_package", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/x86_64/git-1-1-x86_64.pkg.tar.zst",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, gitV1x86_64.data, resp.Body.Bytes())
		})

		t.Run("Get_git_i686_package", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/i686/git-1-1-i686.pkg.tar.zst",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, gitV1i686.data, resp.Body.Bytes())
		})

		t.Run("Get_git_x86_64_package_signature", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/x86_64/git-1-1-x86_64.pkg.tar.zst.sig",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, firstSign, resp.Body.Bytes())
		})

		t.Run("Get_git_i686_package_signature", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/i686/git-1-1-i686.pkg.tar.zst.sig",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, secondSign, resp.Body.Bytes())
		})

		t.Run("Get_any_package_from_x86_64_group", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/x86_64/icons-1-1-any.pkg.tar.zst",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, iconsV1any.data, resp.Body.Bytes())
		})

		t.Run("Get_any_package_from_i686_group", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/i686/icons-1-1-any.pkg.tar.zst",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, iconsV1any.data, resp.Body.Bytes())
		})

		t.Run("Get_x86_64_pacman_database", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/x86_64/user.db.tar.gz",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, V1x86_64database, resp.Body.Bytes())
		})

		t.Run("Get_i686_pacman_database", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/i686/user.db.tar.gz",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, V1i686database, resp.Body.Bytes())
		})
	})

	t.Run("Version_2", func(t *testing.T) {
		t.Run("Push_git_x86_64", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT",
				path.Join(rootURL, "push", "git-2-1-x86_64.pkg.tar.zst", "archlinux"),
				bytes.NewReader(gitV2x86_64.data),
			)

			req = AddBasicAuthHeader(req, user.Name)

			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Push_icons_any", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT",
				path.Join(rootURL, "push", "icons-2-1-any.pkg.tar.zst", "archlinux"),
				bytes.NewReader(iconsV2any.data),
			)

			req = AddBasicAuthHeader(req, user.Name)

			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Get_x86_64_pacman_database", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/x86_64/user2.db.tar.gz",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, V2x86_64database, resp.Body.Bytes())
		})

		t.Run("Get_i686_pacman_database", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET",
				rootURL+"/archlinux/i686/user2.db.tar.gz",
			)

			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, V2i686database, resp.Body.Bytes())
		})

		t.Run("Remove_v2_git_x86_64", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "DELETE", rootURL+"/remove/git/2-1")
			req = AddBasicAuthHeader(req, user.Name)

			MakeRequest(t, req, http.StatusOK)
		})

		t.Run("Remove_v2_icons_any", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "DELETE", rootURL+"/remove/icons/2-1")
			req = AddBasicAuthHeader(req, user.Name)

			MakeRequest(t, req, http.StatusOK)
		})
	})
}

type testArchPackage struct {
	data []byte
	pkg  arch.Package
}

func BuildArchPackage(t *testing.T, name, ver, architecture string) testArchPackage {
	fs := fstest.MapFS{
		"pkginfo": &fstest.MapFile{
			Data: []byte(fmt.Sprintf(
				"pkgname = %s\npkgbase = %s\npkgver = %s\narch = %s\n",
				name, name, ver, architecture,
			)),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
		},
		"mtree": &fstest.MapFile{
			Data:    []byte("test"),
			Mode:    os.ModePerm,
			ModTime: time.Now(),
		},
	}

	pinf, err := fs.Stat("pkginfo")
	assert.NoError(t, err)

	pfile, err := fs.Open("pkginfo")
	assert.NoError(t, err)

	parcname, err := archiver.NameInArchive(pinf, ".PKGINFO", ".PKGINFO")
	assert.NoError(t, err)

	minf, err := fs.Stat("mtree")
	assert.NoError(t, err)

	mfile, err := fs.Open("mtree")
	assert.NoError(t, err)

	marcname, err := archiver.NameInArchive(minf, ".MTREE", ".MTREE")
	assert.NoError(t, err)

	var buf bytes.Buffer

	archive := archiver.NewTarZstd()
	archive.Create(&buf)

	err = archive.Write(archiver.File{
		FileInfo: archiver.FileInfo{
			FileInfo:   pinf,
			CustomName: parcname,
		},
		ReadCloser: pfile,
	})
	assert.NoError(t, errors.Join(pfile.Close(), err))

	err = archive.Write(archiver.File{
		FileInfo: archiver.FileInfo{
			FileInfo:   minf,
			CustomName: marcname,
		},
		ReadCloser: mfile,
	})
	assert.NoError(t, errors.Join(mfile.Close(), archive.Close(), err))

	md5, sha256, size := archPkgParams(buf.Bytes())

	return testArchPackage{
		data: buf.Bytes(),
		pkg: arch.Package{
			Name:    name,
			Version: ver,
			VersionMetadata: arch.VersionMetadata{
				Base: name,
			},
			FileMetadata: arch.FileMetadata{
				CompressedSize: size,
				MD5:            hex.EncodeToString(md5),
				SHA256:         hex.EncodeToString(sha256),
				Arch:           architecture,
			},
		},
	}
}

func archPkgParams(b []byte) ([]byte, []byte, int64) {
	md5 := md5.New()
	sha256 := sha256.New()
	c := counter{bytes.NewReader(b), 0}

	br := bufio.NewReader(io.TeeReader(&c, io.MultiWriter(md5, sha256)))

	io.ReadAll(br)
	return md5.Sum(nil), sha256.Sum(nil), int64(c.n)
}

type counter struct {
	io.Reader
	n int
}

func (w *counter) Read(p []byte) (int, error) {
	n, err := w.Reader.Read(p)
	w.n += n
	return n, err
}

func BuildArchDatabase(pkgs []arch.Package) []byte {
	entries := map[string][]byte{}
	for _, p := range pkgs {
		entries[fmt.Sprintf("%s-%s/desc", p.Name, p.Version)] = []byte(p.Desc())
	}
	b, err := arch.CreatePacmanDb(entries)
	if err != nil {
		panic(err)
	}
	return b.Bytes()
}

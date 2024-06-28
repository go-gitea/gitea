// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
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
		user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		sign = []byte{1, 2, 3, 4, 5, 6}

		pushBatch = []*TestArchPackage{
			BuildArchPackage(t, "git", "1-1", "x86_64"),
			BuildArchPackage(t, "git", "2-1", "x86_64"),
			BuildArchPackage(t, "git", "1-1", "i686"),
			BuildArchPackage(t, "adwaita", "1-1", "any"),
			BuildArchPackage(t, "adwaita", "2-1", "any"),
		}

		removeBatch = []*TestArchPackage{
			BuildArchPackage(t, "curl", "1-1", "x86_64"),
			BuildArchPackage(t, "curl", "2-1", "x86_64"),
			BuildArchPackage(t, "dock", "1-1", "any"),
			BuildArchPackage(t, "dock", "2-1", "any"),
		}

		firstDatabaseBatch = []*TestArchPackage{
			BuildArchPackage(t, "pacman", "1-1", "x86_64"),
			BuildArchPackage(t, "pacman", "1-1", "i686"),
			BuildArchPackage(t, "htop", "1-1", "x86_64"),
			BuildArchPackage(t, "htop", "1-1", "i686"),
			BuildArchPackage(t, "dash", "1-1", "any"),
		}

		secondDatabaseBatch = []*TestArchPackage{
			BuildArchPackage(t, "pacman", "2-1", "x86_64"),
			BuildArchPackage(t, "htop", "2-1", "i686"),
			BuildArchPackage(t, "dash", "2-1", "any"),
		}

		PacmanDBx86 = BuildPacmanDb(t,
			secondDatabaseBatch[0].Pkg,
			firstDatabaseBatch[2].Pkg,
			secondDatabaseBatch[2].Pkg,
		)

		PacmanDBi686 = BuildPacmanDb(t,
			firstDatabaseBatch[0].Pkg,
			secondDatabaseBatch[1].Pkg,
			secondDatabaseBatch[2].Pkg,
		)
	)

	t.Run("PushWithoutSignature", func(t *testing.T) {
		for _, p := range pushBatch {
			t.Run(p.File, func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				url := fmt.Sprintf("/api/packages/%s/arch/parabola", user.Name)

				req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(p.Data)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				pv, err := packages.GetVersionByNameAndVersion(
					db.DefaultContext, user.ID, packages.TypeArch, p.Name, p.Ver,
				)
				assert.NoError(t, err)

				pf, err := packages.GetFileForVersionByName(
					db.DefaultContext, pv.ID, p.File, "parabola",
				)
				assert.NoError(t, err)
				assert.NotNil(t, pf)
			})
		}
	})

	t.Run("GetPackage", func(t *testing.T) {
		for _, p := range pushBatch {
			t.Run(p.File, func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				url := fmt.Sprintf(
					"/api/packages/%s/arch/artix/%s",
					user.Name, base64.RawURLEncoding.EncodeToString(sign),
				)
				req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(p.Data)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				url = fmt.Sprintf(
					"/api/packages/%s/arch/artix/%s/%s",
					user.Name, p.Arch, p.File,
				)
				req = NewRequest(t, "GET", url)
				resp := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, p.Data, resp.Body.Bytes())
			})
		}
	})

	t.Run("GetSignature", func(t *testing.T) {
		for _, p := range pushBatch {
			t.Run(p.File, func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				url := fmt.Sprintf(
					"/api/packages/%s/arch/arco/%s",
					user.Name, base64.RawURLEncoding.EncodeToString(sign),
				)
				req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(p.Data)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				url = fmt.Sprintf(
					"/api/packages/%s/arch/arco/%s/%s.sig",
					user.Name, p.Arch, p.File,
				)
				req = NewRequest(t, "GET", url)
				resp := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, sign, resp.Body.Bytes())
			})
		}
	})

	t.Run("GetPacmanDatabase", func(t *testing.T) {
		for _, p := range firstDatabaseBatch {
			url := fmt.Sprintf(
				"/api/packages/%s/arch/ion/%s",
				user.Name, base64.RawURLEncoding.EncodeToString(sign),
			)
			req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(p.Data)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)
		}

		// Package versions for db are sorted by 'UnixTime', second delay required
		time.Sleep(time.Second)

		for _, p := range secondDatabaseBatch {
			url := fmt.Sprintf(
				"/api/packages/%s/arch/ion/%s",
				user.Name, base64.RawURLEncoding.EncodeToString(sign),
			)
			req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(p.Data)).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)
		}

		t.Run("x86_64", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			url := fmt.Sprintf(
				"/api/packages/%s/arch/ion/x86_64/user.db.tar.gz", user.Name,
			)
			req := NewRequest(t, "GET", url)
			resp := MakeRequest(t, req, http.StatusOK)

			CompareTarGzEntries(t, PacmanDBx86, resp.Body.Bytes())
		})

		t.Run("i686", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			url := fmt.Sprintf(
				"/api/packages/%s/arch/ion/i686/user.db", user.Name,
			)
			req := NewRequest(t, "GET", url)
			resp := MakeRequest(t, req, http.StatusOK)

			CompareTarGzEntries(t, PacmanDBi686, resp.Body.Bytes())
		})
	})

	t.Run("Remove", func(t *testing.T) {
		for _, p := range removeBatch {
			t.Run(p.File, func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				url := fmt.Sprintf(
					"/api/packages/%s/arch/manjaro/%s",
					user.Name, base64.RawURLEncoding.EncodeToString(sign),
				)
				req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(p.Data)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				url = fmt.Sprintf(
					"/api/packages/%s/arch/%s/%s", user.Name, p.Name, p.Ver,
				)
				req = NewRequest(t, "DELETE", url).AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusNoContent)

				_, err := packages.GetVersionByNameAndVersion(
					db.DefaultContext, user.ID, packages.TypeArch, p.Name, p.Ver,
				)
				assert.ErrorIs(t, err, packages.ErrPackageNotExist)
			})
		}
	})
}

type TestArchPackage struct {
	Pkg  arch.Package
	Data []byte
	File string
	Name string
	Ver  string
	Arch string
}

func BuildArchPackage(t *testing.T, name, ver, architecture string) *TestArchPackage {
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

	sha256, size := archPkgParams(buf.Bytes())

	return &TestArchPackage{
		Data: buf.Bytes(),
		Name: name,
		Ver:  ver,
		Arch: architecture,
		File: fmt.Sprintf("%s-%s-%s.pkg.tar.zst", name, ver, architecture),
		Pkg: arch.Package{
			Name:    name,
			Version: ver,
			VersionMetadata: arch.VersionMetadata{
				Base: name,
			},
			FileMetadata: arch.FileMetadata{
				CompressedSize: size,
				SHA256:         hex.EncodeToString(sha256),
				Arch:           architecture,
			},
		},
	}
}

func archPkgParams(b []byte) ([]byte, int64) {
	sha256 := sha256.New()
	c := counter{bytes.NewReader(b), 0}

	br := bufio.NewReader(io.TeeReader(&c, sha256))

	io.ReadAll(br)
	return sha256.Sum(nil), int64(c.n)
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

func BuildPacmanDb(t *testing.T, pkgs ...arch.Package) []byte {
	entries := map[string][]byte{}
	for _, p := range pkgs {
		entries[fmt.Sprintf("%s-%s/desc", p.Name, p.Version)] = []byte(p.Desc())
	}
	b, err := arch.CreatePacmanDb(entries)
	if err != nil {
		assert.NoError(t, err)
		return nil
	}
	return b.Bytes()
}

func CompareTarGzEntries(t *testing.T, expected, actual []byte) {
	fgz, err := gzip.NewReader(bytes.NewReader(expected))
	if err != nil {
		assert.NoError(t, err)
		return
	}
	ftar := tar.NewReader(fgz)

	validatemap := map[string]struct{}{}

	for {
		h, err := ftar.Next()
		if err != nil {
			break
		}

		validatemap[h.Name] = struct{}{}
	}

	sgz, err := gzip.NewReader(bytes.NewReader(actual))
	if err != nil {
		assert.NoError(t, err)
		return
	}
	star := tar.NewReader(sgz)

	for {
		h, err := star.Next()
		if err != nil {
			break
		}

		_, ok := validatemap[h.Name]
		if !ok {
			assert.Fail(t, "Unexpected entry in archive: "+h.Name)
		}
		delete(validatemap, h.Name)
	}

	if len(validatemap) == 0 {
		return
	}

	for e := range validatemap {
		assert.Fail(t, "Entry not found in archive: "+e)
	}
}

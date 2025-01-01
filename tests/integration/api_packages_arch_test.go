// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	arch_module "code.gitea.io/gitea/modules/packages/arch"
	arch_service "code.gitea.io/gitea/services/packages/arch"
	"code.gitea.io/gitea/tests"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/ulikunitz/xz"
)

func TestPackageArch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "gitea-test"
	packageVersion := "1.4.1-r3"

	createPackage := func(compression, name, version, architecture string) []byte {
		var buf bytes.Buffer
		var cw io.WriteCloser
		switch compression {
		case "zst":
			cw, _ = zstd.NewWriter(&buf)
		case "xz":
			cw, _ = xz.NewWriter(&buf)
		case "gz":
			cw = gzip.NewWriter(&buf)
		}
		tw := tar.NewWriter(cw)

		info := []byte(`pkgname = ` + name + `
pkgbase = ` + name + `
pkgver = ` + version + `
pkgdesc = Description
# comment
builddate = 1678834800
size = 8
arch = ` + architecture + `
license = MIT`)

		hdr := &tar.Header{
			Name: ".PKGINFO",
			Mode: 0o600,
			Size: int64(len(info)),
		}
		tw.WriteHeader(hdr)
		tw.Write(info)

		for _, file := range []string{"etc/dummy", "opt/file/bin"} {
			hdr := &tar.Header{
				Name: file,
				Mode: 0o600,
				Size: 4,
			}
			tw.WriteHeader(hdr)
			tw.Write([]byte("test"))
		}

		tw.Close()
		cw.Close()

		return buf.Bytes()
	}

	compressions := []string{"gz", "xz", "zst"}
	repositories := []string{"main", "testing", "with/slash", ""}

	rootURL := fmt.Sprintf("/api/packages/%s/arch", user.Name)

	t.Run("RepositoryKey", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", rootURL+"/repository.key")
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "application/pgp-keys", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Body.String(), "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	})

	contentAarch64Gz := createPackage("gz", packageName, packageVersion, "aarch64")
	for _, compression := range compressions {
		contentAarch64 := createPackage(compression, packageName, packageVersion, "aarch64")
		contentAny := createPackage(compression, packageName+"_"+arch_module.AnyArch, packageVersion, arch_module.AnyArch)

		for _, repository := range repositories {
			t.Run(fmt.Sprintf("[%s,%s]", repository, compression), func(t *testing.T) {
				t.Run("Upload", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					uploadURL := fmt.Sprintf("%s/%s", rootURL, repository)

					req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
					MakeRequest(t, req, http.StatusUnauthorized)

					req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{})).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusBadRequest)

					req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(contentAarch64)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusCreated)

					pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeArch)
					assert.NoError(t, err)
					assert.Len(t, pvs, 1)

					pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
					assert.NoError(t, err)
					assert.Nil(t, pd.SemVer)
					assert.IsType(t, &arch_module.VersionMetadata{}, pd.Metadata)
					assert.Equal(t, packageName, pd.Package.Name)
					assert.Equal(t, packageVersion, pd.Version.Version)

					pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
					assert.NoError(t, err)
					assert.NotEmpty(t, pfs)
					assert.Condition(t, func() bool {
						seen := false
						expectedFilename := fmt.Sprintf("%s-%s-aarch64.pkg.tar.%s", packageName, packageVersion, compression)
						expectedCompositeKey := fmt.Sprintf("%s|aarch64", repository)
						for _, pf := range pfs {
							if pf.Name == expectedFilename && pf.CompositeKey == expectedCompositeKey {
								if seen {
									return false
								}
								seen = true

								assert.True(t, pf.IsLead)

								pfps, err := packages.GetProperties(db.DefaultContext, packages.PropertyTypeFile, pf.ID)
								assert.NoError(t, err)

								for _, pfp := range pfps {
									switch pfp.Name {
									case arch_module.PropertyRepository:
										assert.Equal(t, repository, pfp.Value)
									case arch_module.PropertyArchitecture:
										assert.Equal(t, "aarch64", pfp.Value)
									}
								}
							}
						}
						return seen
					})

					req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(contentAarch64)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusConflict)

					// Add same package with different compression leads to conflict
					req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(contentAarch64Gz)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusConflict)
				})

				readIndexContent := func(r io.Reader) (map[string]string, error) {
					gzr, err := gzip.NewReader(r)
					if err != nil {
						return nil, err
					}

					content := make(map[string]string)

					tr := tar.NewReader(gzr)
					for {
						hd, err := tr.Next()
						if err == io.EOF {
							break
						}
						if err != nil {
							return nil, err
						}

						buf, err := io.ReadAll(tr)
						if err != nil {
							return nil, err
						}

						content[hd.Name] = string(buf)
					}

					return content, nil
				}

				t.Run("Index", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/aarch64/%s", rootURL, repository, arch_service.IndexArchiveFilename))
					resp := MakeRequest(t, req, http.StatusOK)

					content, err := readIndexContent(resp.Body)
					assert.NoError(t, err)

					desc, has := content[fmt.Sprintf("%s-%s/desc", packageName, packageVersion)]
					assert.True(t, has)
					assert.Contains(t, desc, "%FILENAME%\n"+fmt.Sprintf("%s-%s-aarch64.pkg.tar.%s", packageName, packageVersion, compression)+"\n\n")
					assert.Contains(t, desc, "%NAME%\n"+packageName+"\n\n")
					assert.Contains(t, desc, "%VERSION%\n"+packageVersion+"\n\n")
					assert.Contains(t, desc, "%ARCH%\naarch64\n")
					assert.NotContains(t, desc, "%ARCH%\n"+arch_module.AnyArch+"\n")
					assert.Contains(t, desc, "%LICENSE%\nMIT\n")

					files, has := content[fmt.Sprintf("%s-%s/files", packageName, packageVersion)]
					assert.True(t, has)
					assert.Contains(t, files, "%FILES%\netc/dummy\nopt/file/bin\n\n")

					for _, indexFile := range []string{
						arch_service.IndexArchiveFilename,
						arch_service.IndexArchiveFilename + ".tar.gz",
						"index.db",
						"index.db.tar.gz",
						"index.files",
						"index.files.tar.gz",
					} {
						req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/aarch64/%s", rootURL, repository, indexFile))
						MakeRequest(t, req, http.StatusOK)

						req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/aarch64/%s.sig", rootURL, repository, indexFile))
						MakeRequest(t, req, http.StatusOK)
					}
				})

				t.Run("Download", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/aarch64/%s-%s-aarch64.pkg.tar.%s", rootURL, repository, packageName, packageVersion, compression))
					MakeRequest(t, req, http.StatusOK)

					req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/aarch64/%s-%s-aarch64.pkg.tar.%s.sig", rootURL, repository, packageName, packageVersion, compression))
					MakeRequest(t, req, http.StatusOK)
				})

				t.Run("Any", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/%s", rootURL, repository), bytes.NewReader(contentAny)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusCreated)

					req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/aarch64/%s", rootURL, repository, arch_service.IndexArchiveFilename))
					resp := MakeRequest(t, req, http.StatusOK)

					content, err := readIndexContent(resp.Body)
					assert.NoError(t, err)

					desc, has := content[fmt.Sprintf("%s-%s/desc", packageName, packageVersion)]
					assert.True(t, has)
					assert.Contains(t, desc, "%NAME%\n"+packageName+"\n\n")
					assert.Contains(t, desc, "%ARCH%\naarch64\n")

					desc, has = content[fmt.Sprintf("%s-%s/desc", packageName+"_"+arch_module.AnyArch, packageVersion)]
					assert.True(t, has)
					assert.Contains(t, desc, "%NAME%\n"+packageName+"_any\n\n")
					assert.Contains(t, desc, "%ARCH%\n"+arch_module.AnyArch+"\n")

					// "any" architecture package should be available with every architecture requested
					for _, arch := range []string{arch_module.AnyArch, "aarch64", "myarch"} {
						req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s/%s-%s-any.pkg.tar.%s", rootURL, repository, arch, packageName+"_"+arch_module.AnyArch, packageVersion, compression))
						MakeRequest(t, req, http.StatusOK)
					}

					req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s/%s/any", rootURL, repository, packageName+"_"+arch_module.AnyArch, packageVersion)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusNoContent)
				})

				t.Run("Delete", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s/%s/aarch64", rootURL, repository, packageName, packageVersion))
					MakeRequest(t, req, http.StatusUnauthorized)

					req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s/%s/aarch64", rootURL, repository, packageName, packageVersion)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusNoContent)

					// Deleting the last file of an architecture should remove that index
					req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/aarch64/%s", rootURL, repository, arch_service.IndexArchiveFilename))
					MakeRequest(t, req, http.StatusNotFound)
				})
			})
		}
	}
}

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
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	debian_module "code.gitea.io/gitea/modules/packages/debian"
	"code.gitea.io/gitea/tests"

	"github.com/blakesmith/ar"
	"github.com/stretchr/testify/assert"
)

func TestPackageDebian(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "gitea"
	packageVersion := "1.0.3"
	packageDescription := "Package Description"

	createArchive := func(name, version, architecture string) io.Reader {
		var cbuf bytes.Buffer
		zw := gzip.NewWriter(&cbuf)
		tw := tar.NewWriter(zw)
		tw.WriteHeader(&tar.Header{
			Name: "control",
			Mode: 0o600,
			Size: 50,
		})
		fmt.Fprintf(tw, "Package: %s\nVersion: %s\nArchitecture: %s\nDescription: %s\n", name, version, architecture, packageDescription)
		tw.Close()
		zw.Close()

		var buf bytes.Buffer
		aw := ar.NewWriter(&buf)
		aw.WriteGlobalHeader()
		hdr := &ar.Header{
			Name: "control.tar.gz",
			Mode: 0o600,
			Size: int64(cbuf.Len()),
		}
		aw.WriteHeader(hdr)
		aw.Write(cbuf.Bytes())
		return &buf
	}

	distributions := []string{"test", "gitea"}
	components := []string{"main", "stable"}
	architectures := []string{"all", "amd64"}

	rootURL := fmt.Sprintf("/api/packages/%s/debian", user.Name)

	t.Run("RepositoryKey", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", rootURL+"/repository.key")
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "application/pgp-keys", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Body.String(), "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	})

	for _, distribution := range distributions {
		t.Run(fmt.Sprintf("[Distribution:%s]", distribution), func(t *testing.T) {
			for _, component := range components {
				for _, architecture := range architectures {
					t.Run(fmt.Sprintf("[Component:%s,Architecture:%s]", component, architecture), func(t *testing.T) {
						t.Run("Upload", func(t *testing.T) {
							defer tests.PrintCurrentTest(t)()

							uploadURL := fmt.Sprintf("%s/pool/%s/%s/upload", rootURL, distribution, component)

							req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
							MakeRequest(t, req, http.StatusUnauthorized)

							req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
							AddBasicAuthHeader(req, user.Name)
							MakeRequest(t, req, http.StatusBadRequest)

							req = NewRequestWithBody(t, "PUT", uploadURL, createArchive("", "", ""))
							AddBasicAuthHeader(req, user.Name)
							MakeRequest(t, req, http.StatusBadRequest)

							req = NewRequestWithBody(t, "PUT", uploadURL, createArchive(packageName, packageVersion, architecture))
							AddBasicAuthHeader(req, user.Name)
							MakeRequest(t, req, http.StatusCreated)

							pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeDebian)
							assert.NoError(t, err)
							assert.Len(t, pvs, 1)

							pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
							assert.NoError(t, err)
							assert.Nil(t, pd.SemVer)
							assert.IsType(t, &debian_module.Metadata{}, pd.Metadata)
							assert.Equal(t, packageName, pd.Package.Name)
							assert.Equal(t, packageVersion, pd.Version.Version)

							pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
							assert.NoError(t, err)
							assert.NotEmpty(t, pfs)
							assert.Condition(t, func() bool {
								seen := false
								expectedFilename := fmt.Sprintf("%s_%s_%s.deb", packageName, packageVersion, architecture)
								expectedCompositeKey := fmt.Sprintf("%s|%s", distribution, component)
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
											case debian_module.PropertyDistribution:
												assert.Equal(t, distribution, pfp.Value)
											case debian_module.PropertyComponent:
												assert.Equal(t, component, pfp.Value)
											case debian_module.PropertyArchitecture:
												assert.Equal(t, architecture, pfp.Value)
											}
										}
									}
								}
								return seen
							})
						})

						t.Run("Download", func(t *testing.T) {
							defer tests.PrintCurrentTest(t)()

							req := NewRequest(t, "GET", fmt.Sprintf("%s/pool/%s/%s/%s_%s_%s.deb", rootURL, distribution, component, packageName, packageVersion, architecture))
							resp := MakeRequest(t, req, http.StatusOK)

							assert.Equal(t, "application/vnd.debian.binary-package", resp.Header().Get("Content-Type"))
						})

						t.Run("Packages", func(t *testing.T) {
							defer tests.PrintCurrentTest(t)()

							url := fmt.Sprintf("%s/dists/%s/%s/binary-%s/Packages", rootURL, distribution, component, architecture)

							req := NewRequest(t, "GET", url)
							resp := MakeRequest(t, req, http.StatusOK)

							body := resp.Body.String()

							assert.Contains(t, body, "Package: "+packageName)
							assert.Contains(t, body, "Version: "+packageVersion)
							assert.Contains(t, body, "Architecture: "+architecture)
							assert.Contains(t, body, fmt.Sprintf("Filename: pool/%s/%s/%s_%s_%s.deb", distribution, component, packageName, packageVersion, architecture))

							req = NewRequest(t, "GET", url+".gz")
							MakeRequest(t, req, http.StatusOK)

							req = NewRequest(t, "GET", url+".xz")
							MakeRequest(t, req, http.StatusOK)

							url = fmt.Sprintf("%s/dists/%s/%s/%s/by-hash/SHA256/%s", rootURL, distribution, component, architecture, base.EncodeSha256(body))
							req = NewRequest(t, "GET", url)
							resp = MakeRequest(t, req, http.StatusOK)

							assert.Equal(t, body, resp.Body.String())
						})
					})
				}
			}

			t.Run("Release", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%s/dists/%s/Release", rootURL, distribution))
				resp := MakeRequest(t, req, http.StatusOK)

				body := resp.Body.String()

				assert.Contains(t, body, "Components: "+strings.Join(components, " "))
				assert.Contains(t, body, "Architectures: "+strings.Join(architectures, " "))

				for _, component := range components {
					for _, architecture := range architectures {
						assert.Contains(t, body, fmt.Sprintf("%s/binary-%s/Packages", component, architecture))
						assert.Contains(t, body, fmt.Sprintf("%s/binary-%s/Packages.gz", component, architecture))
						assert.Contains(t, body, fmt.Sprintf("%s/binary-%s/Packages.xz", component, architecture))
					}
				}

				req = NewRequest(t, "GET", fmt.Sprintf("%s/dists/%s/by-hash/SHA256/%s", rootURL, distribution, base.EncodeSha256(body)))
				resp = MakeRequest(t, req, http.StatusOK)

				assert.Equal(t, body, resp.Body.String())

				req = NewRequest(t, "GET", fmt.Sprintf("%s/dists/%s/Release.gpg", rootURL, distribution))
				resp = MakeRequest(t, req, http.StatusOK)

				assert.Contains(t, resp.Body.String(), "-----BEGIN PGP SIGNATURE-----")

				req = NewRequest(t, "GET", fmt.Sprintf("%s/dists/%s/InRelease", rootURL, distribution))
				resp = MakeRequest(t, req, http.StatusOK)

				assert.Contains(t, resp.Body.String(), "-----BEGIN PGP SIGNED MESSAGE-----")
			})
		})
	}

	t.Run("Delete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		distribution := distributions[0]
		architecture := architectures[0]

		for _, component := range components {
			req := NewRequest(t, "DELETE", fmt.Sprintf("%s/pool/%s/%s/%s/%s/%s", rootURL, distribution, component, packageName, packageVersion, architecture))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", fmt.Sprintf("%s/pool/%s/%s/%s/%s/%s", rootURL, distribution, component, packageName, packageVersion, architecture))
			AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			req = NewRequest(t, "GET", fmt.Sprintf("%s/dists/%s/%s/binary-%s/Packages", rootURL, distribution, component, architecture))
			MakeRequest(t, req, http.StatusNotFound)
		}

		req := NewRequest(t, "GET", fmt.Sprintf("%s/dists/%s/Release", rootURL, distribution))
		resp := MakeRequest(t, req, http.StatusOK)

		body := resp.Body.String()

		assert.Contains(t, body, "Components: "+strings.Join(components, " "))
		assert.Contains(t, body, "Architectures: "+architectures[1])
	})
}

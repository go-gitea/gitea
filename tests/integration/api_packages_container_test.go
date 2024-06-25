// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	container_model "code.gitea.io/gitea/models/packages/container"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	container_module "code.gitea.io/gitea/modules/packages/container"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
)

func TestPackageContainer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadPackage)
	privateUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 31})

	has := func(l packages_model.PackagePropertyList, name string) bool {
		for _, pp := range l {
			if pp.Name == name {
				return true
			}
		}
		return false
	}
	getAllByName := func(l packages_model.PackagePropertyList, name string) []string {
		values := make([]string, 0, len(l))
		for _, pp := range l {
			if pp.Name == name {
				values = append(values, pp.Value)
			}
		}
		return values
	}

	images := []string{"test", "te/st"}
	tags := []string{"latest", "main"}
	multiTag := "multi"

	unknownDigest := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	blobDigest := "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
	blobContent, _ := base64.StdEncoding.DecodeString(`H4sIAAAJbogA/2IYBaNgFIxYAAgAAP//Lq+17wAEAAA=`)

	configDigest := "sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d"
	configContent := `{"architecture":"amd64","config":{"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/true"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"container":"b89fe92a887d55c0961f02bdfbfd8ac3ddf66167db374770d2d9e9fab3311510","container_config":{"Hostname":"b89fe92a887d","Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) ","CMD [\"/true\"]"],"ArgsEscaped":true,"Image":"sha256:9bd8b88dc68b80cffe126cc820e4b52c6e558eb3b37680bfee8e5f3ed7b8c257"},"created":"2022-01-01T00:00:00.000000000Z","docker_version":"20.10.12","history":[{"created":"2022-01-01T00:00:00.000000000Z","created_by":"/bin/sh -c #(nop) COPY file:0e7589b0c800daaf6fa460d2677101e4676dd9491980210cb345480e513f3602 in /true "},{"created":"2022-01-01T00:00:00.000000001Z","created_by":"/bin/sh -c #(nop)  CMD [\"/true\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:0ff3b91bdf21ecdf2f2f3d4372c2098a14dbe06cd678e8f0a85fd4902d00e2e2"]}}`

	manifestDigest := "sha256:4f10484d1c1bb13e3956b4de1cd42db8e0f14a75be1617b60f2de3cd59c803c6"
	manifestContent := `{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d","size":1069},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4","size":32}]}`

	untaggedManifestDigest := "sha256:4305f5f5572b9a426b88909b036e52ee3cf3d7b9c1b01fac840e90747f56623d"
	untaggedManifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageManifest + `","config":{"mediaType":"application/vnd.docker.container.image.v1+json","digest":"sha256:4607e093bec406eaadb6f3a340f63400c9d3a7038680744c406903766b938f0d","size":1069},"layers":[{"mediaType":"application/vnd.docker.image.rootfs.diff.tar.gzip","digest":"sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4","size":32}]}`

	indexManifestDigest := "sha256:bab112d6efb9e7f221995caaaa880352feb5bd8b1faf52fae8d12c113aa123ec"
	indexManifestContent := `{"schemaVersion":2,"mediaType":"` + oci.MediaTypeImageIndex + `","manifests":[{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","digest":"` + manifestDigest + `","platform":{"os":"linux","architecture":"arm","variant":"v7"}},{"mediaType":"` + oci.MediaTypeImageManifest + `","digest":"` + untaggedManifestDigest + `","platform":{"os":"linux","architecture":"arm64","variant":"v8"}}]}`

	anonymousToken := ""
	userToken := ""

	t.Run("Authenticate", func(t *testing.T) {
		type TokenResponse struct {
			Token string `json:"token"`
		}

		defaultAuthenticateValues := []string{`Bearer realm="` + setting.AppURL + `v2/token",service="container_registry",scope="*"`}

		t.Run("Anonymous", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL))
			resp := MakeRequest(t, req, http.StatusUnauthorized)

			assert.ElementsMatch(t, defaultAuthenticateValues, resp.Header().Values("WWW-Authenticate"))

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL))
			resp = MakeRequest(t, req, http.StatusOK)

			tokenResponse := &TokenResponse{}
			DecodeJSON(t, resp, &tokenResponse)

			assert.NotEmpty(t, tokenResponse.Token)

			anonymousToken = fmt.Sprintf("Bearer %s", tokenResponse.Token)

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL)).
				AddTokenAuth(anonymousToken)
			MakeRequest(t, req, http.StatusOK)

			defer test.MockVariableValue(&setting.Service.RequireSignInView, true)()

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL))
			MakeRequest(t, req, http.StatusUnauthorized)

			defer test.MockVariableValue(&setting.AppURL, "https://domain:8443/sub-path/")()
			defer test.MockVariableValue(&setting.AppSubURL, "/sub-path")()
			req = NewRequest(t, "GET", "/v2")
			resp = MakeRequest(t, req, http.StatusUnauthorized)
			assert.Equal(t, `Bearer realm="https://domain:8443/v2/token",service="container_registry",scope="*"`, resp.Header().Get("WWW-Authenticate"))
		})

		t.Run("User", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL))
			resp := MakeRequest(t, req, http.StatusUnauthorized)

			assert.ElementsMatch(t, defaultAuthenticateValues, resp.Header().Values("WWW-Authenticate"))

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2/token", setting.AppURL)).
				AddBasicAuth(user.Name)
			resp = MakeRequest(t, req, http.StatusOK)

			tokenResponse := &TokenResponse{}
			DecodeJSON(t, resp, &tokenResponse)

			assert.NotEmpty(t, tokenResponse.Token)

			userToken = fmt.Sprintf("Bearer %s", tokenResponse.Token)

			req = NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL)).
				AddTokenAuth(userToken)
			MakeRequest(t, req, http.StatusOK)
		})
	})

	t.Run("DetermineSupport", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%sv2", setting.AppURL)).
			AddTokenAuth(userToken)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, "registry/2.0", resp.Header().Get("Docker-Distribution-Api-Version"))
	})

	for _, image := range images {
		t.Run(fmt.Sprintf("[Image:%s]", image), func(t *testing.T) {
			url := fmt.Sprintf("%sv2/%s/%s", setting.AppURL, user.Name, image)

			t.Run("UploadBlob/Monolithic", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads", url)).
					AddTokenAuth(anonymousToken)
				MakeRequest(t, req, http.StatusUnauthorized)

				req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, unknownDigest), bytes.NewReader(blobContent)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusBadRequest)

				req = NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, blobDigest), bytes.NewReader(blobContent)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, blobDigest), resp.Header().Get("Location"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))

				pv, err := packages_model.GetInternalVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, container_model.UploadVersion)
				assert.NoError(t, err)

				pfs, err := packages_model.GetFilesByVersionID(db.DefaultContext, pv.ID)
				assert.NoError(t, err)
				assert.Len(t, pfs, 1)

				pb, err := packages_model.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
				assert.NoError(t, err)
				assert.EqualValues(t, len(blobContent), pb.Size)
			})

			t.Run("UploadBlob/Chunked", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads", url)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusAccepted)

				uuid := resp.Header().Get("Docker-Upload-Uuid")
				assert.NotEmpty(t, uuid)

				pbu, err := packages_model.GetBlobUploadByID(db.DefaultContext, uuid)
				assert.NoError(t, err)
				assert.EqualValues(t, 0, pbu.BytesReceived)

				uploadURL := resp.Header().Get("Location")
				assert.NotEmpty(t, uploadURL)

				req = NewRequestWithBody(t, "PATCH", setting.AppURL+uploadURL[1:]+"000", bytes.NewReader(blobContent)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusNotFound)

				req = NewRequestWithBody(t, "PATCH", setting.AppURL+uploadURL[1:], bytes.NewReader(blobContent)).
					AddTokenAuth(userToken).
					SetHeader("Content-Range", "1-10")
				MakeRequest(t, req, http.StatusRequestedRangeNotSatisfiable)

				contentRange := fmt.Sprintf("0-%d", len(blobContent)-1)
				req.SetHeader("Content-Range", contentRange)
				resp = MakeRequest(t, req, http.StatusAccepted)

				assert.Equal(t, uuid, resp.Header().Get("Docker-Upload-Uuid"))
				assert.Equal(t, contentRange, resp.Header().Get("Range"))

				uploadURL = resp.Header().Get("Location")

				req = NewRequest(t, "GET", setting.AppURL+uploadURL[1:]).
					AddTokenAuth(userToken)
				resp = MakeRequest(t, req, http.StatusNoContent)

				assert.Equal(t, uuid, resp.Header().Get("Docker-Upload-Uuid"))
				assert.Equal(t, fmt.Sprintf("0-%d", len(blobContent)), resp.Header().Get("Range"))

				pbu, err = packages_model.GetBlobUploadByID(db.DefaultContext, uuid)
				assert.NoError(t, err)
				assert.EqualValues(t, len(blobContent), pbu.BytesReceived)

				req = NewRequest(t, "PUT", fmt.Sprintf("%s?digest=%s", setting.AppURL+uploadURL[1:], blobDigest)).
					AddTokenAuth(userToken)
				resp = MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, blobDigest), resp.Header().Get("Location"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))

				t.Run("Cancel", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads", url)).
						AddTokenAuth(userToken)
					resp := MakeRequest(t, req, http.StatusAccepted)

					uuid := resp.Header().Get("Docker-Upload-Uuid")
					assert.NotEmpty(t, uuid)

					uploadURL := resp.Header().Get("Location")
					assert.NotEmpty(t, uploadURL)

					req = NewRequest(t, "GET", setting.AppURL+uploadURL[1:]).
						AddTokenAuth(userToken)
					resp = MakeRequest(t, req, http.StatusNoContent)

					assert.Equal(t, uuid, resp.Header().Get("Docker-Upload-Uuid"))
					assert.Equal(t, "0-0", resp.Header().Get("Range"))

					req = NewRequest(t, "DELETE", setting.AppURL+uploadURL[1:]).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNoContent)

					req = NewRequest(t, "GET", setting.AppURL+uploadURL[1:]).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNotFound)
				})
			})

			t.Run("UploadBlob/Mount", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				privateBlobDigest := "sha256:6ccce4863b70f258d691f59609d31b4502e1ba5199942d3bc5d35d17a4ce771d"
				req := NewRequestWithBody(t, "POST", fmt.Sprintf("%sv2/%s/%s/blobs/uploads?digest=%s", setting.AppURL, privateUser.Name, image, privateBlobDigest), strings.NewReader("gitea")).
					AddBasicAuth(privateUser.Name)
				MakeRequest(t, req, http.StatusCreated)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s", url, unknownDigest)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusAccepted)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s", url, privateBlobDigest)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusAccepted)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s", url, blobDigest)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, blobDigest), resp.Header().Get("Location"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s&from=%s", url, unknownDigest, "unknown/image")).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusAccepted)

				req = NewRequest(t, "POST", fmt.Sprintf("%s/blobs/uploads?mount=%s&from=%s/%s", url, blobDigest, user.Name, image)).
					AddTokenAuth(userToken)
				resp = MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, fmt.Sprintf("/v2/%s/%s/blobs/%s", user.Name, image, blobDigest), resp.Header().Get("Location"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))
			})

			for _, tag := range tags {
				t.Run(fmt.Sprintf("[Tag:%s]", tag), func(t *testing.T) {
					t.Run("UploadManifest", func(t *testing.T) {
						defer tests.PrintCurrentTest(t)()

						req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, configDigest), strings.NewReader(configContent)).
							AddTokenAuth(userToken)
						MakeRequest(t, req, http.StatusCreated)

						req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(manifestContent)).
							AddTokenAuth(anonymousToken).
							SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
						MakeRequest(t, req, http.StatusUnauthorized)

						req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(manifestContent)).
							AddTokenAuth(userToken).
							SetHeader("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
						resp := MakeRequest(t, req, http.StatusCreated)

						assert.Equal(t, manifestDigest, resp.Header().Get("Docker-Content-Digest"))

						pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, tag)
						assert.NoError(t, err)

						pd, err := packages_model.GetPackageDescriptor(db.DefaultContext, pv)
						assert.NoError(t, err)
						assert.Nil(t, pd.SemVer)
						assert.Equal(t, image, pd.Package.Name)
						assert.Equal(t, tag, pd.Version.Version)
						assert.ElementsMatch(t, []string{strings.ToLower(user.LowerName + "/" + image)}, getAllByName(pd.PackageProperties, container_module.PropertyRepository))
						assert.True(t, has(pd.VersionProperties, container_module.PropertyManifestTagged))

						assert.IsType(t, &container_module.Metadata{}, pd.Metadata)
						metadata := pd.Metadata.(*container_module.Metadata)
						assert.Equal(t, container_module.TypeOCI, metadata.Type)
						assert.Len(t, metadata.ImageLayers, 2)
						assert.Empty(t, metadata.Manifests)

						assert.Len(t, pd.Files, 3)
						for _, pfd := range pd.Files {
							switch pfd.File.Name {
							case container_model.ManifestFilename:
								assert.True(t, pfd.File.IsLead)
								assert.Equal(t, "application/vnd.docker.distribution.manifest.v2+json", pfd.Properties.GetByName(container_module.PropertyMediaType))
								assert.Equal(t, manifestDigest, pfd.Properties.GetByName(container_module.PropertyDigest))
							case strings.Replace(configDigest, ":", "_", 1):
								assert.False(t, pfd.File.IsLead)
								assert.Equal(t, "application/vnd.docker.container.image.v1+json", pfd.Properties.GetByName(container_module.PropertyMediaType))
								assert.Equal(t, configDigest, pfd.Properties.GetByName(container_module.PropertyDigest))
							case strings.Replace(blobDigest, ":", "_", 1):
								assert.False(t, pfd.File.IsLead)
								assert.Equal(t, "application/vnd.docker.image.rootfs.diff.tar.gzip", pfd.Properties.GetByName(container_module.PropertyMediaType))
								assert.Equal(t, blobDigest, pfd.Properties.GetByName(container_module.PropertyDigest))
							default:
								assert.FailNow(t, "unknown file: %s", pfd.File.Name)
							}
						}

						req = NewRequest(t, "GET", fmt.Sprintf("%s/manifests/%s", url, tag)).
							AddTokenAuth(userToken)
						MakeRequest(t, req, http.StatusOK)

						pv, err = packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, tag)
						assert.NoError(t, err)
						assert.EqualValues(t, 1, pv.DownloadCount)

						// Overwrite existing tag should keep the download count
						req = NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, tag), strings.NewReader(manifestContent)).
							AddTokenAuth(userToken).
							SetHeader("Content-Type", oci.MediaTypeImageManifest)
						MakeRequest(t, req, http.StatusCreated)

						pv, err = packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, tag)
						assert.NoError(t, err)
						assert.EqualValues(t, 1, pv.DownloadCount)
					})

					t.Run("HeadManifest", func(t *testing.T) {
						defer tests.PrintCurrentTest(t)()

						req := NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/unknown-tag", url)).
							AddTokenAuth(userToken)
						MakeRequest(t, req, http.StatusNotFound)

						req = NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/%s", url, tag)).
							AddTokenAuth(userToken)
						resp := MakeRequest(t, req, http.StatusOK)

						assert.Equal(t, fmt.Sprintf("%d", len(manifestContent)), resp.Header().Get("Content-Length"))
						assert.Equal(t, manifestDigest, resp.Header().Get("Docker-Content-Digest"))
					})

					t.Run("GetManifest", func(t *testing.T) {
						defer tests.PrintCurrentTest(t)()

						req := NewRequest(t, "GET", fmt.Sprintf("%s/manifests/unknown-tag", url)).
							AddTokenAuth(userToken)
						MakeRequest(t, req, http.StatusNotFound)

						req = NewRequest(t, "GET", fmt.Sprintf("%s/manifests/%s", url, tag)).
							AddTokenAuth(userToken)
						resp := MakeRequest(t, req, http.StatusOK)

						assert.Equal(t, fmt.Sprintf("%d", len(manifestContent)), resp.Header().Get("Content-Length"))
						assert.Equal(t, oci.MediaTypeImageManifest, resp.Header().Get("Content-Type"))
						assert.Equal(t, manifestDigest, resp.Header().Get("Docker-Content-Digest"))
						assert.Equal(t, manifestContent, resp.Body.String())
					})
				})
			}

			t.Run("UploadUntaggedManifest", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, untaggedManifestDigest), strings.NewReader(untaggedManifestContent)).
					AddTokenAuth(userToken).
					SetHeader("Content-Type", oci.MediaTypeImageManifest)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, untaggedManifestDigest, resp.Header().Get("Docker-Content-Digest"))

				req = NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/%s", url, untaggedManifestDigest)).
					AddTokenAuth(userToken)
				resp = MakeRequest(t, req, http.StatusOK)

				assert.Equal(t, fmt.Sprintf("%d", len(untaggedManifestContent)), resp.Header().Get("Content-Length"))
				assert.Equal(t, untaggedManifestDigest, resp.Header().Get("Docker-Content-Digest"))

				pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, untaggedManifestDigest)
				assert.NoError(t, err)

				pd, err := packages_model.GetPackageDescriptor(db.DefaultContext, pv)
				assert.NoError(t, err)
				assert.Nil(t, pd.SemVer)
				assert.Equal(t, image, pd.Package.Name)
				assert.Equal(t, untaggedManifestDigest, pd.Version.Version)
				assert.ElementsMatch(t, []string{strings.ToLower(user.LowerName + "/" + image)}, getAllByName(pd.PackageProperties, container_module.PropertyRepository))
				assert.False(t, has(pd.VersionProperties, container_module.PropertyManifestTagged))

				assert.IsType(t, &container_module.Metadata{}, pd.Metadata)

				assert.Len(t, pd.Files, 3)
				for _, pfd := range pd.Files {
					if pfd.File.Name == container_model.ManifestFilename {
						assert.True(t, pfd.File.IsLead)
						assert.Equal(t, oci.MediaTypeImageManifest, pfd.Properties.GetByName(container_module.PropertyMediaType))
						assert.Equal(t, untaggedManifestDigest, pfd.Properties.GetByName(container_module.PropertyDigest))
					}
				}
			})

			t.Run("UploadIndexManifest", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequestWithBody(t, "PUT", fmt.Sprintf("%s/manifests/%s", url, multiTag), strings.NewReader(indexManifestContent)).
					AddTokenAuth(userToken).
					SetHeader("Content-Type", oci.MediaTypeImageIndex)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, indexManifestDigest, resp.Header().Get("Docker-Content-Digest"))

				pv, err := packages_model.GetVersionByNameAndVersion(db.DefaultContext, user.ID, packages_model.TypeContainer, image, multiTag)
				assert.NoError(t, err)

				pd, err := packages_model.GetPackageDescriptor(db.DefaultContext, pv)
				assert.NoError(t, err)
				assert.Nil(t, pd.SemVer)
				assert.Equal(t, image, pd.Package.Name)
				assert.Equal(t, multiTag, pd.Version.Version)
				assert.ElementsMatch(t, []string{strings.ToLower(user.LowerName + "/" + image)}, getAllByName(pd.PackageProperties, container_module.PropertyRepository))
				assert.True(t, has(pd.VersionProperties, container_module.PropertyManifestTagged))

				assert.ElementsMatch(t, []string{manifestDigest, untaggedManifestDigest}, getAllByName(pd.VersionProperties, container_module.PropertyManifestReference))

				assert.IsType(t, &container_module.Metadata{}, pd.Metadata)
				metadata := pd.Metadata.(*container_module.Metadata)
				assert.Equal(t, container_module.TypeOCI, metadata.Type)
				assert.Len(t, metadata.Manifests, 2)
				assert.Condition(t, func() bool {
					for _, m := range metadata.Manifests {
						switch m.Platform {
						case "linux/arm/v7":
							assert.Equal(t, manifestDigest, m.Digest)
							assert.EqualValues(t, 1524, m.Size)
						case "linux/arm64/v8":
							assert.Equal(t, untaggedManifestDigest, m.Digest)
							assert.EqualValues(t, 1514, m.Size)
						default:
							return false
						}
					}
					return true
				})

				assert.Len(t, pd.Files, 1)
				assert.True(t, pd.Files[0].File.IsLead)
				assert.Equal(t, oci.MediaTypeImageIndex, pd.Files[0].Properties.GetByName(container_module.PropertyMediaType))
				assert.Equal(t, indexManifestDigest, pd.Files[0].Properties.GetByName(container_module.PropertyDigest))
			})

			t.Run("HeadBlob", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", url, unknownDigest)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusNotFound)

				req = NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", url, blobDigest)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusOK)

				assert.Equal(t, fmt.Sprintf("%d", len(blobContent)), resp.Header().Get("Content-Length"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))

				req = NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", url, blobDigest)).
					AddTokenAuth(anonymousToken)
				MakeRequest(t, req, http.StatusOK)
			})

			t.Run("GetBlob", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%s/blobs/%s", url, unknownDigest)).
					AddTokenAuth(userToken)
				MakeRequest(t, req, http.StatusNotFound)

				req = NewRequest(t, "GET", fmt.Sprintf("%s/blobs/%s", url, blobDigest)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusOK)

				assert.Equal(t, fmt.Sprintf("%d", len(blobContent)), resp.Header().Get("Content-Length"))
				assert.Equal(t, blobDigest, resp.Header().Get("Docker-Content-Digest"))
				assert.Equal(t, blobContent, resp.Body.Bytes())
			})

			t.Run("GetTagList", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				cases := []struct {
					URL          string
					ExpectedTags []string
					ExpectedLink string
				}{
					{
						URL:          fmt.Sprintf("%s/tags/list", url),
						ExpectedTags: []string{"latest", "main", "multi"},
						ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=multi>; rel="next"`, user.Name, image),
					},
					{
						URL:          fmt.Sprintf("%s/tags/list?n=0", url),
						ExpectedTags: []string{},
						ExpectedLink: "",
					},
					{
						URL:          fmt.Sprintf("%s/tags/list?n=2", url),
						ExpectedTags: []string{"latest", "main"},
						ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=main&n=2>; rel="next"`, user.Name, image),
					},
					{
						URL:          fmt.Sprintf("%s/tags/list?last=main", url),
						ExpectedTags: []string{"multi"},
						ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=multi>; rel="next"`, user.Name, image),
					},
					{
						URL:          fmt.Sprintf("%s/tags/list?n=1&last=latest", url),
						ExpectedTags: []string{"main"},
						ExpectedLink: fmt.Sprintf(`</v2/%s/%s/tags/list?last=main&n=1>; rel="next"`, user.Name, image),
					},
				}

				for _, c := range cases {
					req := NewRequest(t, "GET", c.URL).
						AddTokenAuth(userToken)
					resp := MakeRequest(t, req, http.StatusOK)

					type TagList struct {
						Name string   `json:"name"`
						Tags []string `json:"tags"`
					}

					tagList := &TagList{}
					DecodeJSON(t, resp, &tagList)

					assert.Equal(t, user.Name+"/"+image, tagList.Name)
					assert.Equal(t, c.ExpectedTags, tagList.Tags)
					assert.Equal(t, c.ExpectedLink, resp.Header().Get("Link"))
				}

				req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/packages/%s?type=container&q=%s", user.Name, image)).
					AddTokenAuth(token)
				resp := MakeRequest(t, req, http.StatusOK)

				var apiPackages []*api.Package
				DecodeJSON(t, resp, &apiPackages)
				assert.Len(t, apiPackages, 4) // "latest", "main", "multi", "sha256:..."
			})

			t.Run("Delete", func(t *testing.T) {
				t.Run("Blob", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "DELETE", fmt.Sprintf("%s/blobs/%s", url, blobDigest)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusAccepted)

					req = NewRequest(t, "HEAD", fmt.Sprintf("%s/blobs/%s", url, blobDigest)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNotFound)
				})

				t.Run("ManifestByDigest", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "DELETE", fmt.Sprintf("%s/manifests/%s", url, untaggedManifestDigest)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusAccepted)

					req = NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/%s", url, untaggedManifestDigest)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNotFound)
				})

				t.Run("ManifestByTag", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "DELETE", fmt.Sprintf("%s/manifests/%s", url, multiTag)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusAccepted)

					req = NewRequest(t, "HEAD", fmt.Sprintf("%s/manifests/%s", url, multiTag)).
						AddTokenAuth(userToken)
					MakeRequest(t, req, http.StatusNotFound)
				})
			})
		})
	}

	// https://github.com/go-gitea/gitea/issues/19586
	t.Run("ParallelUpload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		url := fmt.Sprintf("%sv2/%s/parallel", setting.AppURL, user.Name)

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)

			content := []byte{byte(i)}
			digest := fmt.Sprintf("sha256:%x", sha256.Sum256(content))

			go func() {
				defer wg.Done()

				req := NewRequestWithBody(t, "POST", fmt.Sprintf("%s/blobs/uploads?digest=%s", url, digest), bytes.NewReader(content)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusCreated)

				assert.Equal(t, digest, resp.Header().Get("Docker-Content-Digest"))
			}()
		}
		wg.Wait()
	})

	t.Run("OwnerNameChange", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		checkCatalog := func(owner string) func(t *testing.T) {
			return func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", fmt.Sprintf("%sv2/_catalog", setting.AppURL)).
					AddTokenAuth(userToken)
				resp := MakeRequest(t, req, http.StatusOK)

				type RepositoryList struct {
					Repositories []string `json:"repositories"`
				}

				repoList := &RepositoryList{}
				DecodeJSON(t, resp, &repoList)

				assert.Len(t, repoList.Repositories, len(images))
				names := make([]string, 0, len(images))
				for _, image := range images {
					names = append(names, strings.ToLower(owner+"/"+image))
				}
				assert.ElementsMatch(t, names, repoList.Repositories)
			}
		}

		t.Run(fmt.Sprintf("Catalog[%s]", user.LowerName), checkCatalog(user.LowerName))

		session := loginUser(t, user.Name)

		newOwnerName := "newUsername"

		req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"_csrf":    GetCSRF(t, session, "/user/settings"),
			"name":     newOwnerName,
			"email":    "user2@example.com",
			"language": "en-US",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		t.Run(fmt.Sprintf("Catalog[%s]", newOwnerName), checkCatalog(newOwnerName))

		req = NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
			"_csrf":    GetCSRF(t, session, "/user/settings"),
			"name":     user.Name,
			"email":    "user2@example.com",
			"language": "en-US",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)
	})
}

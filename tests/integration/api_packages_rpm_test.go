// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	rpm_module "code.gitea.io/gitea/modules/packages/rpm"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/sassoftware/go-rpmutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageRpm(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	packageName := "gitea-test"
	packageVersion := "1.0.2-1"
	packageArchitecture := "x86_64"

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	base64RpmPackageContent := `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`
	rpmPackageContent, err := base64.StdEncoding.DecodeString(base64RpmPackageContent)
	assert.NoError(t, err)

	zr, err := gzip.NewReader(bytes.NewReader(rpmPackageContent))
	assert.NoError(t, err)

	content, err := io.ReadAll(zr)
	assert.NoError(t, err)

	decodeGzipXML := func(t testing.TB, resp *httptest.ResponseRecorder, v any) {
		t.Helper()

		zr, err := gzip.NewReader(resp.Body)
		assert.NoError(t, err)

		assert.NoError(t, xml.NewDecoder(zr).Decode(v))
	}

	rootURL := fmt.Sprintf("/api/packages/%s/rpm", user.Name)

	for _, group := range []string{"", "el9", "el9/stable"} {
		t.Run(fmt.Sprintf("[Group:%s]", group), func(t *testing.T) {
			var groupParts []string
			if group != "" {
				groupParts = strings.Split(group, "/")
			}
			groupURL := strings.Join(append([]string{rootURL}, groupParts...), "/")

			t.Run("RepositoryConfig", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", groupURL+".repo")
				resp := MakeRequest(t, req, http.StatusOK)

				expected := fmt.Sprintf(`[gitea-%s]
name=%s
baseurl=%s
enabled=1
gpgcheck=1
gpgkey=%sapi/packages/%s/rpm/repository.key`,
					strings.Join(append([]string{user.LowerName}, groupParts...), "-"),
					strings.Join(append([]string{user.Name, setting.AppName}, groupParts...), " - "),
					strings.TrimSuffix(setting.AppURL, "/")+groupURL,
					setting.AppURL,
					user.Name,
				)

				assert.Equal(t, expected, resp.Body.String())
			})

			t.Run("RepositoryKey", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", rootURL+"/repository.key")
				resp := MakeRequest(t, req, http.StatusOK)

				assert.Equal(t, "application/pgp-keys", resp.Header().Get("Content-Type"))
				assert.Contains(t, resp.Body.String(), "-----BEGIN PGP PUBLIC KEY BLOCK-----")
			})

			t.Run("Upload", func(t *testing.T) {
				url := groupURL + "/upload"

				req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(content))
				MakeRequest(t, req, http.StatusUnauthorized)

				req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeRpm)
				assert.NoError(t, err)
				assert.Len(t, pvs, 1)

				pd, err := packages.GetPackageDescriptor(t.Context(), pvs[0])
				assert.NoError(t, err)
				assert.Nil(t, pd.SemVer)
				assert.IsType(t, &rpm_module.VersionMetadata{}, pd.Metadata)
				assert.Equal(t, packageName, pd.Package.Name)
				assert.Equal(t, packageVersion, pd.Version.Version)

				pfs, err := packages.GetFilesByVersionID(t.Context(), pvs[0].ID)
				assert.NoError(t, err)
				assert.Len(t, pfs, 1)
				assert.Equal(t, fmt.Sprintf("%s-%s.%s.rpm", packageName, packageVersion, packageArchitecture), pfs[0].Name)
				assert.True(t, pfs[0].IsLead)

				pb, err := packages.GetBlobByID(t.Context(), pfs[0].BlobID)
				assert.NoError(t, err)
				assert.Equal(t, int64(len(content)), pb.Size)

				req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusConflict)
			})

			t.Run("Download", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				// download the package without the file name
				req := NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s", groupURL, packageName, packageVersion, packageArchitecture))
				resp := MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, content, resp.Body.Bytes())

				// download the package with a file name (it can be anything)
				req = NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s/any-file-name", groupURL, packageName, packageVersion, packageArchitecture))
				resp = MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, content, resp.Body.Bytes())
			})

			t.Run("Repository", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				url := groupURL + "/repodata"

				req := NewRequest(t, "HEAD", url+"/dummy.xml")
				MakeRequest(t, req, http.StatusNotFound)

				req = NewRequest(t, "GET", url+"/dummy.xml")
				MakeRequest(t, req, http.StatusNotFound)

				t.Run("repomd.xml", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req = NewRequest(t, "HEAD", url+"/repomd.xml")
					MakeRequest(t, req, http.StatusOK)

					req = NewRequest(t, "GET", url+"/repomd.xml")
					resp := MakeRequest(t, req, http.StatusOK)

					type Repomd struct {
						XMLName  xml.Name `xml:"repomd"`
						Xmlns    string   `xml:"xmlns,attr"`
						XmlnsRpm string   `xml:"xmlns:rpm,attr"`
						Data     []struct {
							Type     string `xml:"type,attr"`
							Checksum struct {
								Value string `xml:",chardata"`
								Type  string `xml:"type,attr"`
							} `xml:"checksum"`
							OpenChecksum struct {
								Value string `xml:",chardata"`
								Type  string `xml:"type,attr"`
							} `xml:"open-checksum"`
							Location struct {
								Href string `xml:"href,attr"`
							} `xml:"location"`
							Timestamp int64 `xml:"timestamp"`
							Size      int64 `xml:"size"`
							OpenSize  int64 `xml:"open-size"`
						} `xml:"data"`
					}

					var result Repomd
					decodeXML(t, resp, &result)

					assert.Len(t, result.Data, 3)
					for _, d := range result.Data {
						assert.Equal(t, "sha256", d.Checksum.Type)
						assert.NotEmpty(t, d.Checksum.Value)
						assert.Equal(t, "sha256", d.OpenChecksum.Type)
						assert.NotEmpty(t, d.OpenChecksum.Value)
						assert.NotEqual(t, d.Checksum.Value, d.OpenChecksum.Value)
						assert.Greater(t, d.OpenSize, d.Size)

						switch d.Type {
						case "primary":
							assert.EqualValues(t, 722, d.Size)
							assert.EqualValues(t, 1759, d.OpenSize)
							assert.Equal(t, "repodata/primary.xml.gz", d.Location.Href)
						case "filelists":
							assert.EqualValues(t, 257, d.Size)
							assert.EqualValues(t, 326, d.OpenSize)
							assert.Equal(t, "repodata/filelists.xml.gz", d.Location.Href)
						case "other":
							assert.EqualValues(t, 306, d.Size)
							assert.EqualValues(t, 394, d.OpenSize)
							assert.Equal(t, "repodata/other.xml.gz", d.Location.Href)
						}
					}
				})

				t.Run("repomd.xml.asc", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req = NewRequest(t, "GET", url+"/repomd.xml.asc")
					resp := MakeRequest(t, req, http.StatusOK)

					assert.Contains(t, resp.Body.String(), "-----BEGIN PGP SIGNATURE-----")
				})

				t.Run("primary.xml.gz", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req = NewRequest(t, "GET", url+"/primary.xml.gz")
					resp := MakeRequest(t, req, http.StatusOK)

					type EntryList struct {
						Entries []*rpm_module.Entry `xml:"entry"`
					}

					type Metadata struct {
						XMLName      xml.Name `xml:"metadata"`
						Xmlns        string   `xml:"xmlns,attr"`
						XmlnsRpm     string   `xml:"xmlns:rpm,attr"`
						PackageCount int      `xml:"packages,attr"`
						Packages     []struct {
							XMLName      xml.Name `xml:"package"`
							Type         string   `xml:"type,attr"`
							Name         string   `xml:"name"`
							Architecture string   `xml:"arch"`
							Version      struct {
								Epoch   string `xml:"epoch,attr"`
								Version string `xml:"ver,attr"`
								Release string `xml:"rel,attr"`
							} `xml:"version"`
							Checksum struct {
								Checksum string `xml:",chardata"`
								Type     string `xml:"type,attr"`
								Pkgid    string `xml:"pkgid,attr"`
							} `xml:"checksum"`
							Summary     string `xml:"summary"`
							Description string `xml:"description"`
							Packager    string `xml:"packager"`
							URL         string `xml:"url"`
							Time        struct {
								File  uint64 `xml:"file,attr"`
								Build uint64 `xml:"build,attr"`
							} `xml:"time"`
							Size struct {
								Package   int64  `xml:"package,attr"`
								Installed uint64 `xml:"installed,attr"`
								Archive   uint64 `xml:"archive,attr"`
							} `xml:"size"`
							Location struct {
								Href string `xml:"href,attr"`
							} `xml:"location"`
							Format struct {
								License   string             `xml:"license"`
								Vendor    string             `xml:"vendor"`
								Group     string             `xml:"group"`
								Buildhost string             `xml:"buildhost"`
								Sourcerpm string             `xml:"sourcerpm"`
								Provides  EntryList          `xml:"provides"`
								Requires  EntryList          `xml:"requires"`
								Conflicts EntryList          `xml:"conflicts"`
								Obsoletes EntryList          `xml:"obsoletes"`
								Files     []*rpm_module.File `xml:"file"`
							} `xml:"format"`
						} `xml:"package"`
					}

					var result Metadata
					decodeGzipXML(t, resp, &result)

					assert.Equal(t, 1, result.PackageCount)
					assert.Len(t, result.Packages, 1)
					p := result.Packages[0]
					assert.Equal(t, "rpm", p.Type)
					assert.Equal(t, packageName, p.Name)
					assert.Equal(t, packageArchitecture, p.Architecture)
					assert.Equal(t, "YES", p.Checksum.Pkgid)
					assert.Equal(t, "sha256", p.Checksum.Type)
					assert.Equal(t, "f1d5d2ffcbe4a7568e98b864f40d923ecca084e9b9bcd5977ed6521c46d3fa4c", p.Checksum.Checksum)
					assert.Equal(t, "https://gitea.io", p.URL)
					assert.EqualValues(t, len(content), p.Size.Package)
					assert.EqualValues(t, 13, p.Size.Installed)
					assert.EqualValues(t, 272, p.Size.Archive)
					assert.Equal(t, fmt.Sprintf("package/%s/%s/%s/%s", packageName, packageVersion, packageArchitecture, fmt.Sprintf("%s-%s.%s.rpm", packageName, packageVersion, packageArchitecture)), p.Location.Href)
					f := p.Format
					assert.Equal(t, "MIT", f.License)
					assert.Len(t, f.Provides.Entries, 2)
					assert.Len(t, f.Requires.Entries, 7)
					assert.Empty(t, f.Conflicts.Entries)
					assert.Empty(t, f.Obsoletes.Entries)
					assert.Len(t, f.Files, 1)
				})

				t.Run("filelists.xml.gz", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req = NewRequest(t, "GET", url+"/filelists.xml.gz")
					resp := MakeRequest(t, req, http.StatusOK)

					type Filelists struct {
						XMLName      xml.Name `xml:"filelists"`
						Xmlns        string   `xml:"xmlns,attr"`
						PackageCount int      `xml:"packages,attr"`
						Packages     []struct {
							Pkgid        string `xml:"pkgid,attr"`
							Name         string `xml:"name,attr"`
							Architecture string `xml:"arch,attr"`
							Version      struct {
								Epoch   string `xml:"epoch,attr"`
								Version string `xml:"ver,attr"`
								Release string `xml:"rel,attr"`
							} `xml:"version"`
							Files []*rpm_module.File `xml:"file"`
						} `xml:"package"`
					}

					var result Filelists
					decodeGzipXML(t, resp, &result)

					assert.Equal(t, 1, result.PackageCount)
					assert.Len(t, result.Packages, 1)
					p := result.Packages[0]
					assert.NotEmpty(t, p.Pkgid)
					assert.Equal(t, packageName, p.Name)
					assert.Equal(t, packageArchitecture, p.Architecture)
					assert.Len(t, p.Files, 1)
					f := p.Files[0]
					assert.Equal(t, "/usr/local/bin/hello", f.Path)
				})

				t.Run("other.xml.gz", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req = NewRequest(t, "GET", url+"/other.xml.gz")
					resp := MakeRequest(t, req, http.StatusOK)

					type Other struct {
						XMLName      xml.Name `xml:"otherdata"`
						Xmlns        string   `xml:"xmlns,attr"`
						PackageCount int      `xml:"packages,attr"`
						Packages     []struct {
							Pkgid        string `xml:"pkgid,attr"`
							Name         string `xml:"name,attr"`
							Architecture string `xml:"arch,attr"`
							Version      struct {
								Epoch   string `xml:"epoch,attr"`
								Version string `xml:"ver,attr"`
								Release string `xml:"rel,attr"`
							} `xml:"version"`
							Changelogs []*rpm_module.Changelog `xml:"changelog"`
						} `xml:"package"`
					}

					var result Other
					decodeGzipXML(t, resp, &result)

					assert.Equal(t, 1, result.PackageCount)
					assert.Len(t, result.Packages, 1)
					p := result.Packages[0]
					assert.NotEmpty(t, p.Pkgid)
					assert.Equal(t, packageName, p.Name)
					assert.Equal(t, packageArchitecture, p.Architecture)
					assert.Len(t, p.Changelogs, 1)
					c := p.Changelogs[0]
					assert.Equal(t, "KN4CK3R <dummy@gitea.io>", c.Author)
					assert.EqualValues(t, 1678276800, c.Date)
					assert.Equal(t, "- Changelog message.", c.Text)
				})
			})

			t.Run("Errata", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				type updateInfo struct {
					XMLName xml.Name             `xml:"updates"`
					Xmlns   string               `xml:"xmlns,attr"`
					Updates []*rpm_module.Update `xml:"update"`
				}

				errataURL := fmt.Sprintf("%s/package/%s/%s/errata", groupURL, packageName, packageVersion)

				advisory := rpm_module.Update{
					From:        "security@example.com",
					Status:      "stable",
					Type:        "security",
					Version:     "1.0",
					ID:          "CVE-2023-1234",
					Title:       "Test Security Update",
					Severity:    "Important",
					Description: "This is a test security update.",
					References: []*rpm_module.Reference{
						{
							Href:  "https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2023-1234",
							ID:    "CVE-2023-1234",
							Title: "CVE-2023-1234",
							Type:  "cve",
						},
					},
					PkgList: []*rpm_module.Collection{
						{
							Short: "el9",
							Packages: []*rpm_module.UpdatePackage{
								{
									Arch:     packageArchitecture,
									Name:     packageName,
									Release:  "1",
									Src:      "gitea-test-1.0.2-1.src.rpm",
									Version:  "1.0.2",
									Filename: fmt.Sprintf("%s-%s.%s.rpm", packageName, packageVersion, packageArchitecture),
								},
							},
						},
					},
				}

				t.Run("Success", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					updates := []*rpm_module.Update{&advisory}
					body, err := json.Marshal(updates)
					assert.NoError(t, err)

					req := NewRequestWithBody(t, "POST", errataURL, bytes.NewReader(body)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusOK)

					url := groupURL + "/repodata"

					// Check repomd.xml contains updateinfo
					req = NewRequest(t, "GET", url+"/repomd.xml")
					resp := MakeRequest(t, req, http.StatusOK)

					type testRepoData struct {
						Type string `xml:"type,attr"`
					}
					var repomd struct {
						Data []*testRepoData `xml:"data"`
					}
					err = xml.NewDecoder(resp.Body).Decode(&repomd)
					require.NoError(t, err)

					found := slices.IndexFunc(repomd.Data, func(s *testRepoData) bool {
						return s.Type == "updateinfo"
					}) >= 0
					assert.True(t, found, "updateinfo not found in repomd.xml")

					// Now check updateinfo.xml.gz
					req = NewRequest(t, "GET", url+"/updateinfo.xml.gz")
					resp = MakeRequest(t, req, http.StatusOK)

					var result updateInfo
					decodeGzipXML(t, resp, &result)

					assert.Equal(t, "http://linux.duke.edu/metadata/updateinfo", result.Xmlns)
					assert.Len(t, result.Updates, 1)
					assert.Equal(t, "CVE-2023-1234", result.Updates[0].ID)
					assert.NotEmpty(t, result.Updates[0].Issued.Date)
					assert.NotEmpty(t, result.Updates[0].Updated.Date)
				})

				t.Run("InvalidJSON", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequestWithBody(t, "POST", errataURL, strings.NewReader("invalid json")).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusBadRequest)
				})

				t.Run("NullElementsInJSON", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					// Send a payload with null inside arrays
					payload := `[
						{
							"id": "CVE-2023-5678",
							"from": "security@example.com",
							"status": "stable",
							"type": "security",
							"version": "1.0",
							"title": "Test Null Elements",
							"severity": "Important",
							"description": "Test null elements",
							"pkg_list": [
								null,
								{
									"short": "el9",
									"packages": [
										null,
										{
											"arch": "x86_64",
											"name": "gitea",
											"release": "1",
											"src": "gitea-1.0.0-1.src.rpm",
											"version": "1.0.0",
											"filename": "gitea-1.0.0-1.x86_64.rpm"
										}
									]
								}
							]
						}
					]`

					req := NewRequestWithBody(t, "POST", errataURL, strings.NewReader(payload)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusOK)

					// Verify it was stored correctly (skipping nulls)
					url := groupURL + "/repodata"
					req = NewRequest(t, "GET", url+"/updateinfo.xml.gz")
					resp := MakeRequest(t, req, http.StatusOK)

					var result updateInfo
					decodeGzipXML(t, resp, &result)

					// We need to find the new advisory CVE-2023-5678
					var newAdvisory *rpm_module.Update
					for _, u := range result.Updates {
						if u.ID == "CVE-2023-5678" {
							newAdvisory = u
							break
						}
					}
					assert.NotNil(t, newAdvisory)
					assert.Len(t, newAdvisory.PkgList, 1)
					assert.Equal(t, "el9", newAdvisory.PkgList[0].Short)
					assert.Len(t, newAdvisory.PkgList[0].Packages, 1)
					assert.Equal(t, "gitea", newAdvisory.PkgList[0].Packages[0].Name)
				})

				t.Run("PackageNotFound", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					badURL := fmt.Sprintf("%s/package/%s/non-existent-version/errata", groupURL, packageName)
					updates := []*rpm_module.Update{&advisory}
					body, err := json.Marshal(updates)
					assert.NoError(t, err)

					req := NewRequestWithBody(t, "POST", badURL, bytes.NewReader(body)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusNotFound)
				})

				t.Run("MergeAdvisories", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					// Upload a second advisory with the same ID but a different package
					advisory2 := advisory
					advisory2.PkgList = []*rpm_module.Collection{
						{
							Short: "el9",
							Packages: []*rpm_module.UpdatePackage{
								{
									Arch:     packageArchitecture,
									Name:     "another-package",
									Release:  "1",
									Src:      "another-package-1.0.0-1.src.rpm",
									Version:  "1.0.0",
									Filename: "another-package-1.0.0-1.x86_64.rpm",
								},
							},
						},
					}

					updates := []*rpm_module.Update{&advisory2}
					body, err := json.Marshal(updates)
					assert.NoError(t, err)

					req := NewRequestWithBody(t, "POST", errataURL, bytes.NewReader(body)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusOK)

					// Check updateinfo.xml.gz again
					url := groupURL + "/repodata"
					req = NewRequest(t, "GET", url+"/updateinfo.xml.gz")
					resp := MakeRequest(t, req, http.StatusOK)

					var result updateInfo
					decodeGzipXML(t, resp, &result)

					var targetUpdate *rpm_module.Update
					for _, u := range result.Updates {
						if u.ID == "CVE-2023-1234" {
							targetUpdate = u
							break
						}
					}
					assert.NotNil(t, targetUpdate)
					// Verify that package lists are merged into the same collection
					assert.Len(t, targetUpdate.PkgList, 1)
					assert.Len(t, targetUpdate.PkgList[0].Packages, 2)
					assert.Equal(t, "another-package", result.Updates[0].PkgList[0].Packages[1].Name)
				})

				t.Run("NewCollection", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					// Upload a third advisory with the same ID but a different collection
					advisory3 := advisory
					advisory3.PkgList = []*rpm_module.Collection{
						{
							Short: "el8",
							Packages: []*rpm_module.UpdatePackage{
								{
									Arch:     packageArchitecture,
									Name:     packageName,
									Release:  "1",
									Src:      "gitea-test-1.0.2-1.src.rpm",
									Version:  "1.0.2",
									Filename: fmt.Sprintf("%s-%s.%s.rpm", packageName, packageVersion, packageArchitecture),
								},
							},
						},
					}

					updates := []*rpm_module.Update{&advisory3}
					body, err := json.Marshal(updates)
					assert.NoError(t, err)

					req := NewRequestWithBody(t, "POST", errataURL, bytes.NewReader(body)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusOK)

					// Check updateinfo.xml.gz again
					url := groupURL + "/repodata"
					req = NewRequest(t, "GET", url+"/updateinfo.xml.gz")
					resp := MakeRequest(t, req, http.StatusOK)

					var result updateInfo
					decodeGzipXML(t, resp, &result)

					var targetUpdate *rpm_module.Update
					for _, u := range result.Updates {
						if u.ID == "CVE-2023-1234" {
							targetUpdate = u
							break
						}
					}
					assert.NotNil(t, targetUpdate)
					// Verify that we now have 2 collections
					assert.Len(t, targetUpdate.PkgList, 2)
					// We need to be careful with order, map iteration is random
					// Let's check both exist
					shorts := []string{targetUpdate.PkgList[0].Short, targetUpdate.PkgList[1].Short}
					assert.Contains(t, shorts, "el9")
					assert.Contains(t, shorts, "el8")
				})

				t.Run("Idempotency", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					updates := []*rpm_module.Update{&advisory}
					body, err := json.Marshal(updates)
					assert.NoError(t, err)

					// Post twice
					req := NewRequestWithBody(t, "POST", errataURL, bytes.NewReader(body)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusOK)

					req = NewRequestWithBody(t, "POST", errataURL, bytes.NewReader(body)).
						AddBasicAuth(user.Name)
					MakeRequest(t, req, http.StatusOK)

					// Check updateinfo.xml.gz
					url := groupURL + "/repodata"
					req = NewRequest(t, "GET", url+"/updateinfo.xml.gz")
					resp := MakeRequest(t, req, http.StatusOK)

					var result updateInfo
					decodeGzipXML(t, resp, &result)

					var targetUpdate *rpm_module.Update
					for _, u := range result.Updates {
						if u.ID == "CVE-2023-1234" {
							targetUpdate = u
							break
						}
					}
					assert.NotNil(t, targetUpdate)
					assert.Len(t, targetUpdate.PkgList, 2)

					var el9Coll *rpm_module.Collection
					for _, coll := range targetUpdate.PkgList {
						if coll.Short == "el9" {
							el9Coll = coll
							break
						}
					}
					assert.NotNil(t, el9Coll)
					assert.Len(t, el9Coll.Packages, 2)
				})
			})

			t.Run("Delete", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "DELETE", fmt.Sprintf("%s/package/%s/%s/%s", groupURL, packageName, packageVersion, packageArchitecture))
				MakeRequest(t, req, http.StatusUnauthorized)

				req = NewRequest(t, "DELETE", fmt.Sprintf("%s/package/%s/%s/%s", groupURL, packageName, packageVersion, packageArchitecture)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusNoContent)

				pvs, err := packages.GetVersionsByPackageType(t.Context(), user.ID, packages.TypeRpm)
				assert.NoError(t, err)
				assert.Empty(t, pvs)
				req = NewRequest(t, "DELETE", fmt.Sprintf("%s/package/%s/%s/%s", groupURL, packageName, packageVersion, packageArchitecture)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusNotFound)
			})

			t.Run("UploadSign", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				url := groupURL + "/upload?sign=true"
				req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(content)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				gpgReq := NewRequest(t, "GET", rootURL+"/repository.key")
				gpgResp := MakeRequest(t, gpgReq, http.StatusOK)
				pub, err := openpgp.ReadArmoredKeyRing(gpgResp.Body)
				require.NoError(t, err)

				rpmFileName := fmt.Sprintf("%s-%s.%s.rpm", packageName, packageVersion, packageArchitecture)
				req = NewRequest(t, "GET", fmt.Sprintf("%s/package/%s/%s/%s/%s", groupURL, packageName, packageVersion, packageArchitecture, rpmFileName))
				resp := MakeRequest(t, req, http.StatusOK)

				_, sigs, err := rpmutils.Verify(resp.Body, pub)
				require.NoError(t, err)
				require.NotEmpty(t, sigs)

				req = NewRequest(t, "DELETE", fmt.Sprintf("%s/package/%s/%s/%s", groupURL, packageName, packageVersion, packageArchitecture)).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusNoContent)
			})
		})
	}
}

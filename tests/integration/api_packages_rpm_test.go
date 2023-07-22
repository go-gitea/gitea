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
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	rpm_module "code.gitea.io/gitea/modules/packages/rpm"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageRpm(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	rootURL := fmt.Sprintf("/api/packages/%s/rpm", user.Name)

	type PkgInfo struct {
		packageName         string
		packageVersion      string
		packageArchitecture string
	}

	pkgsInfo := []PkgInfo{
		{
			packageName:         "gitea-test",
			packageVersion:      "1.0.2-1",
			packageArchitecture: "x86_64",
		},
		{
			packageName:         "minio-client",
			packageVersion:      "2023.07.11-1.el9",
			packageArchitecture: "x86_64",
		},
	}
	pkgDists := []string{
		"default",
		"el9",
	}
	base64PkgContents := []string{
		`H4sICFayB2QCAGdpdGVhLXRlc3QtMS4wLjItMS14ODZfNjQucnBtAO2YV4gTQRjHJzl7wbNhhxVF
VNwk2zd2PdvZ9Sxnd3Z3NllNsmF3o6congVFsWFHRWwIImIXfRER0QcRfPBJEXvvBQvWSfZTT0VQ
8TF/MuU33zcz3+zOJGEe73lyuQBRBWKWRzDrEddjuVAkxLMc+lsFUOWfm5bvvReAalWECg/TsivU
dyKa0U61aVnl6wj0Uxe4nc8F92hZiaYE8CO/P0r7/Quegr0c7M/AvoCaGZEIWNGUqMHrhhGROIUT
Zc7gOAOraoQzCNZ0WdU0HpEI5jiB4zlek3gT85wqCBomhomxoGCs8wImWMImbxqKgXVNUKKaqShR
STKVKK9glFUNcf2g+/t27xs16v5x/eyOKftVGlIhyiuvvPLKK6+88sorr7zyyiuvvPKCO5HPnz+v
pGVhhXsTsFVeSstuWR9anwU+Bk3Vch5wTwL3JkHg+8C1gR8A169wj1KdpobAj4HbAT+Be5VewE+h
fz/g52AvBX4N9vHAb4AnA7+F8ePAH8BuA38ELgf+BLzQ50oIeBlw0OdAOXAlP57AGuCsbwGtbgCu
DrwRuAb4bwau6T/PwFbgWsDXgWuD/y3gOmC/B1wI/Bi4AcT3Arih3z9YCNzI9w9m/YKUG4Nd9N9z
pSZgHwrcFPgccFt//OADGE+F/q+Ao+D/FrijzwV1gbv4/QvaAHcFDgF3B5aB+wB3Be7rz1dQCtwP
eDxwMcw3GbgU7AasdwzYE8DjwT4L/CeAvRx4IvBCYA3iWQds+FzpDjABfghsAj8BTgA/A/b8+StX
A84A1wKe5s9fuRB4JpzHZv55rL8a/Dv49vpn/PErR4BvQX8Z+Db4l2W5CH2/f0W5+1fEoeFDBzFp
rE/FMcK4mWQSOzN+aDOIqztW2rPsFKIyqh7sQERR42RVMSKihnzVHlQ8Ag0YLBYNEIajkhmuR5Io
7nlpt2M4nJs0ZNkoYaUyZahMlSfJImr1n1WjFVNCPCaTZgYNGdGL8YN2mX8WHfA/C7ViHJK0pxHG
SrkeTiSI4T+7ubf85yrzRCQRQ5EVxVAjvIBVRY/KRFAVReIkhfARSddNSceayQkGliIKb0q8RAxJ
5QWNVxHIsW3Pz369bw+5jh5y0klE9Znqm0dF57b0HbGy2A5lVUBTZZrqZjdUjYoprFmpsBtHP5d0
+ISltS2yk2mHuC4x+lgJMhgnidvuqy3b0suK0bm+tw3FMxI2zjm7/fA0MtQhplX2s7nYLZ2ZC0yg
CxJZDokhORTJlrlcCvG5OieGBERlVCs7CfuS6WzQ/T2j+9f92BWxTFEcp2IkYccYGp2LYySEfreq
irue4WRF5XkpKovw2wgpq2rZBI8bQZkzxEkiYaNwxnXCCVvHidzIiB3CM2yMYdNWmjDsaLovaE4c
x3a6mLaTxB7rEj3jWN4M2p7uwPaa1GfI8BHFfcZMKhkycnhR7y781/a+A4t7FpWWTupRUtKbegwZ
XMKwJinTSe70uhRcj55qNu3YHtE922Fdz7FTMTq9Q3TbMdiYrrPudMvT44S6u2miu138eC0tTN9D
2CFGHHtQsHHsGCRFDFbXuT9wx6mUTZfseydlkWZeJkW6xOgYjqXT+LA7I6XHaUx2xmUzqelWymA9
rCXI9+D1BHbjsITssqhBNysw0tOWjcpmIh6+aViYPfftw8ZSGfRVPUqKiosZj5R5qGmk/8AjjRbZ
d8b3vvngdPHx3HvMeCarIk7VVSwbgoZVkceEVyOmyUmGxBGNYDVKSFSOGlIkGqWnUZFkiY/wsmhK
Mu0UFYgZ/bYnuvn/vz4wtCz8qMwsHUvP0PX3tbYFUctAPdrY6tiiDtcCddDECahx7SuVNP5dpmb5
9tMDyaXb7OAlk5acuPn57ss9mw6Wym0m1Fq2cej7tUt2LL4/b8enXU2fndk+fvv57ndnt55/cQob
7tpp/pEjDS7cGPZ6BY430+7danDq6f42Nw49b9F7zp6BiKpJb9s5P0AYN2+L159cnrur636rx+v1
7ae1K28QbMMcqI8CqwIrgwg9nTOp8Oj9q81plUY7ZuwXN8Vvs8wbAAA=`,
		`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`,
	}

	contents := make([][]byte, 2)

	for k := range pkgDists {
		rpmPackageContent, err := base64.StdEncoding.DecodeString(base64PkgContents[k])
		assert.NoError(t, err)
		zr, err := gzip.NewReader(bytes.NewReader(rpmPackageContent))
		assert.NoError(t, err)
		content, err := io.ReadAll(zr)
		assert.NoError(t, err)
		contents[k] = content
	}
	t.Run("RepositoryConfig", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		for _, dist := range pkgDists {
			req := NewRequest(t, "GET", rootURL+"/"+dist+".repo")
			resp := MakeRequest(t, req, http.StatusOK)

			expected := fmt.Sprintf(`[gitea-%s-%s]
name=%s - %s - %s
baseurl=%sapi/packages/%s/rpm/%s
enabled=1
gpgcheck=1
gpgkey=%sapi/packages/%s/rpm/repository.key`, user.Name, dist, user.Name, setting.AppName, dist, setting.AppURL, user.Name, dist, setting.AppURL, user.Name)
			assert.Equal(t, expected, resp.Body.String())

		}
	})

	t.Run("RepositoryKey", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", rootURL+"/repository.key")
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "application/pgp-keys", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Body.String(), "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	})

	t.Run("Upload", func(t *testing.T) {
		for index, dist := range pkgDists {
			url := rootURL + "/" + dist + "/upload"
			req := NewRequestWithBody(t, "PUT", url, bytes.NewReader(contents[index]))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(contents[index]))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusCreated)

			pvs, err := packages.GetVersionsByPackageName(db.DefaultContext, user.ID, packages.TypeRpm, pkgsInfo[index].packageName)
			assert.NoError(t, err)
			assert.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
			assert.NoError(t, err)
			assert.Nil(t, pd.SemVer)
			assert.IsType(t, &rpm_module.VersionMetadata{}, pd.Metadata)
			assert.Equal(t, pkgsInfo[index].packageName, pd.Package.Name)
			assert.Equal(t, pkgsInfo[index].packageVersion, pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			assert.NoError(t, err)
			assert.Len(t, pfs, 1)
			assert.Equal(t, fmt.Sprintf("%s-%s.%s.rpm", pkgsInfo[index].packageName, pkgsInfo[index].packageVersion, pkgsInfo[index].packageArchitecture), pfs[0].Name)
			assert.True(t, pfs[0].IsLead)

			pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
			assert.NoError(t, err)
			assert.Equal(t, int64(len(contents[index])), pb.Size)

			req = NewRequestWithBody(t, "PUT", url, bytes.NewReader(contents[index]))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusConflict)

		}
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/package/%s/%s/%s", rootURL, pkgDists[0], pkgsInfo[0].packageName, pkgsInfo[0].packageVersion, pkgsInfo[0].packageArchitecture))
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Equal(t, contents[0], resp.Body.Bytes())

		errReq := NewRequest(t, "GET", fmt.Sprintf("%s/%s/package/%s/%s/%s", rootURL, pkgDists[0], pkgsInfo[1].packageName, pkgsInfo[1].packageVersion, pkgsInfo[1].packageArchitecture))
		MakeRequest(t, errReq, http.StatusNotFound)

		otherReq := NewRequest(t, "GET", fmt.Sprintf("%s/%s/package/%s/%s/%s", rootURL, pkgDists[1], pkgsInfo[1].packageName, pkgsInfo[1].packageVersion, pkgsInfo[1].packageArchitecture))
		otherResp := MakeRequest(t, otherReq, http.StatusOK)
		assert.Equal(t, contents[1], otherResp.Body.Bytes())
	})

	t.Run("Repository", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		url := rootURL + "/default/repodata"

		req := NewRequest(t, "GET", url+"/dummy.xml")
		MakeRequest(t, req, http.StatusNotFound)

		t.Run("repomd.xml", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

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
					assert.EqualValues(t, 718, d.Size)
					assert.EqualValues(t, 1729, d.OpenSize)
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

		decodeGzipXML := func(t testing.TB, resp *httptest.ResponseRecorder, v any) {
			t.Helper()

			zr, err := gzip.NewReader(resp.Body)
			assert.NoError(t, err)

			assert.NoError(t, xml.NewDecoder(zr).Decode(v))
		}

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

			assert.EqualValues(t, 1, result.PackageCount)
			assert.Len(t, result.Packages, 1)
			p := result.Packages[0]
			assert.Equal(t, "rpm", p.Type)
			assert.Equal(t, pkgsInfo[0].packageName, p.Name)
			assert.Equal(t, pkgsInfo[0].packageArchitecture, p.Architecture)
			assert.Equal(t, "YES", p.Checksum.Pkgid)
			assert.Equal(t, "sha256", p.Checksum.Type)
			assert.Equal(t, "f1d5d2ffcbe4a7568e98b864f40d923ecca084e9b9bcd5977ed6521c46d3fa4c", p.Checksum.Checksum)
			assert.Equal(t, "https://gitea.io", p.URL)
			assert.EqualValues(t, len(contents[0]), p.Size.Package)
			assert.EqualValues(t, 13, p.Size.Installed)
			assert.EqualValues(t, 272, p.Size.Archive)
			assert.Equal(t, fmt.Sprintf("package/%s/%s/%s", pkgsInfo[0].packageName, pkgsInfo[0].packageVersion, pkgsInfo[0].packageArchitecture), p.Location.Href)
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

			assert.EqualValues(t, 1, result.PackageCount)
			assert.Len(t, result.Packages, 1)
			p := result.Packages[0]
			assert.NotEmpty(t, p.Pkgid)
			assert.Equal(t, pkgsInfo[0].packageName, p.Name)
			assert.Equal(t, pkgsInfo[0].packageArchitecture, p.Architecture)
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

			assert.EqualValues(t, 1, result.PackageCount)
			assert.Len(t, result.Packages, 1)
			p := result.Packages[0]
			assert.NotEmpty(t, p.Pkgid)
			assert.Equal(t, pkgsInfo[0].packageName, p.Name)
			assert.Equal(t, pkgsInfo[0].packageArchitecture, p.Architecture)
			assert.Len(t, p.Changelogs, 1)
			c := p.Changelogs[0]
			assert.Equal(t, "KN4CK3R <dummy@gitea.io>", c.Author)
			assert.EqualValues(t, 1678276800, c.Date)
			assert.Equal(t, "- Changelog message.", c.Text)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		for index, dist := range pkgDists {

			req := NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/package/%s/%s/%s", rootURL, dist, pkgsInfo[index].packageName, pkgsInfo[index].packageVersion, pkgsInfo[index].packageArchitecture))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/package/%s/%s/%s", rootURL, dist, pkgsInfo[index].packageName, pkgsInfo[index].packageVersion, pkgsInfo[index].packageArchitecture))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			pvs, err := packages.GetVersionsByPackageName(db.DefaultContext, user.ID, packages.TypeRpm, pkgsInfo[index].packageName)
			assert.NoError(t, err)
			assert.Empty(t, pvs)

			req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/package/%s/%s/%s", rootURL, dist, pkgsInfo[index].packageName, pkgsInfo[index].packageVersion, pkgsInfo[index].packageArchitecture))
			req = AddBasicAuthHeader(req, user.Name)
			MakeRequest(t, req, http.StatusNotFound)
		}
	})
}

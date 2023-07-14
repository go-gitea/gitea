// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	alpine_module "code.gitea.io/gitea/modules/packages/alpine"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageAlpine(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	packageName := "gitea-test"
	packageVersion := "1.4.1-r3"

	base64AlpinePackageContent := `H4sIAAAAAAACA9ML9nT30wsKdtTLzjNJzjYuckjPLElN1DUzMUxMNTa11CsqTtQrKE1ioAAYAIGZ
iQmYBgJ02hDENjQxMTAzMzQ1MTVjMDA0MTQ1ZlAwYKADKC0uSSxSUGAYoWDm4sZZtypv75+q2fVT
POD1bKkFB22ms+g1z+H4dk7AhC3HwUSj9EbT0Rk3Dn55dHxy/K7Q+Nl/i+L7Z036ypcRvvpZuMiN
s7wbZL/klqRGGshv9Gi0qHTgTZfw3HytnJdx9c3NTRp/PHn+Z50uq2pjkilzjtpfd+uzQMw1M7cY
i9RXJasnT2M+vDXCesLK7MilJt8sGplj4xUlLMUun9SzY+phFpxWxRXa06AseV9WvzH3jtGGoL5A
vQkea+VKPj5R+Cb461tIk97qpa9nJYsJujTNl2B/J1P52H/D2rPr/j19uU8p7cMSq5tmXk51ReXl
F/Yddr9XsMpEwFKlXSPo3QSGwnCOG8y2uadjm6ui998WYXNYubjg78N3a7bnXjhrl5fB8voI++LI
1FP5W44e2xf4Ou2wrtyic1Onz7MzMV5ksuno2V/LVG4eN/15X/n2/2vJ2VV+T68aT327dOrhd6e6
q5Y0V82Y83tdqkFa8TW2BvGCZ0ds/iibHVpzKuPcuSULO63/bNmfrnhjWqXzhMSXTb5Cv4vPaxSL
8LFMdqmxbN7+Y+Yi0ZyZhz4UxexLuHHFd1VFvk+kwvniq3P+f9rh52InWnL8Lpvedcecoh1GFSc5
xZ9VBGex2V269HZfwxSVCvP35wQfi2xKX+lYMXtF48n1R65O2PLWpm69RdESMa79dlrTGazsZacu
MbMLeSSScPORZde76/MBV6SFJAAEAAAfiwgAAAAAAAID7VRLaxsxEN6zfoUgZ++OVq+1aUIhUDeY
pKa49FhmJdkW3ofRysXpr69220t9SCk0gZJ+IGaY56eBmbxY4/m9Q+vCUOTr1fLu4d2H7O8CEpQQ
k0y4lAClypgQoBSTQqoMGBMgMnrOXgCnIWJIVLLXCcaoib5110CSij/V7D9eCZ5p5f9o/5VkF/tf
MqUzCi+5/6Hv41Nxv/Nffu4fwRVdus4FjM7S+pFiffKNpTxnkMMsALmin5PnHgMtS8rkgvGFBPpp
c0tLKDk5HnYdto5e052PDmfRDXE0fnUh2VgucjYLU5h1g0mm5RhGNymMrtEccOfIKTTJsY/xOCyK
YqqT+74gExWbmI2VlJ6LeQUcyPFH2lh/9SBuV/wjfXPohDnw8HZKviGD/zYmCZgrgsHsk36u1Bcl
SB/8zne/0jV92/qYbKRF38X0niiemN2QxhvXDWOL+7tNGhGeYt+m22mwaR6pddGZNM8FSeRxj8PY
X7PaqdqAVlqWXHKnmQGmK43VlqNlILRilbBSMI2jV5Vbu5XGSVsDyGc7yd8B/gK2qgAIAAAfiwgA
AAAAAAID7dNNSgMxGAbg7MSCOxcu5wJOv0x+OlkU7K5QoYXqVsxMMihlKMwP1Fu48QQewCN4DfEQ
egUz4sYuFKEtFN9n870hWSSQN+7P7GrsrfNV3Y9dW5Z3bNMo0FJ+zmB9EhcJ41KS1lxJpRnxbsWi
FduBtm5sFa7C/ifOo7y5Lf2QeiHar6jTaDSbnF5Mp+fzOL/x+aJuy3g+HvGhs8JY4b3yOpMZOZEo
lRW+MEoTTw3ZwqU0INNjsAe2VPk/9b/L3/s/kIKzqOtk+IbJGTtmr+bx7WoxOUoun98frk/un14O
Djfa/2q5bH4699v++uMAAAAAAAAAAAAAAAAAAAAAAHbgA/eXQh8AKAAA`
	content, err := base64.StdEncoding.DecodeString(base64AlpinePackageContent)
	assert.NoError(t, err)

	branches := []string{"v3.16", "v3.17", "v3.18"}
	repositories := []string{"main", "testing"}

	rootURL := fmt.Sprintf("/api/packages/%s/alpine", user.Name)

	t.Run("RepositoryKey", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", rootURL+"/key")
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "application/x-pem-file", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Body.String(), "-----BEGIN PUBLIC KEY-----")
	})

	for _, branch := range branches {
		for _, repository := range repositories {
			t.Run(fmt.Sprintf("[Branch:%s,Repository:%s]", branch, repository), func(t *testing.T) {
				t.Run("Upload", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					uploadURL := fmt.Sprintf("%s/%s/%s", rootURL, branch, repository)

					req := NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
					MakeRequest(t, req, http.StatusUnauthorized)

					req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader([]byte{}))
					AddBasicAuthHeader(req, user.Name)
					MakeRequest(t, req, http.StatusBadRequest)

					req = NewRequestWithBody(t, "PUT", uploadURL, bytes.NewReader(content))
					AddBasicAuthHeader(req, user.Name)
					MakeRequest(t, req, http.StatusCreated)

					pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeAlpine)
					assert.NoError(t, err)
					assert.Len(t, pvs, 1)

					pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
					assert.NoError(t, err)
					assert.Nil(t, pd.SemVer)
					assert.IsType(t, &alpine_module.VersionMetadata{}, pd.Metadata)
					assert.Equal(t, packageName, pd.Package.Name)
					assert.Equal(t, packageVersion, pd.Version.Version)

					pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
					assert.NoError(t, err)
					assert.NotEmpty(t, pfs)
					assert.Condition(t, func() bool {
						seen := false
						expectedFilename := fmt.Sprintf("%s-%s.apk", packageName, packageVersion)
						expectedCompositeKey := fmt.Sprintf("%s|%s|x86_64", branch, repository)
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
									case alpine_module.PropertyBranch:
										assert.Equal(t, branch, pfp.Value)
									case alpine_module.PropertyRepository:
										assert.Equal(t, repository, pfp.Value)
									case alpine_module.PropertyArchitecture:
										assert.Equal(t, "x86_64", pfp.Value)
									}
								}
							}
						}
						return seen
					})
				})

				t.Run("Index", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					url := fmt.Sprintf("%s/%s/%s/x86_64/APKINDEX.tar.gz", rootURL, branch, repository)

					req := NewRequest(t, "GET", url)
					resp := MakeRequest(t, req, http.StatusOK)

					assert.Condition(t, func() bool {
						br := bufio.NewReader(resp.Body)

						gzr, err := gzip.NewReader(br)
						assert.NoError(t, err)

						for {
							gzr.Multistream(false)

							tr := tar.NewReader(gzr)
							for {
								hd, err := tr.Next()
								if err == io.EOF {
									break
								}
								assert.NoError(t, err)

								if hd.Name == "APKINDEX" {
									buf, err := io.ReadAll(tr)
									assert.NoError(t, err)

									s := string(buf)

									assert.Contains(t, s, "C:Q1/se1PjO94hYXbfpNR1/61hVORIc=\n")
									assert.Contains(t, s, "P:"+packageName+"\n")
									assert.Contains(t, s, "V:"+packageVersion+"\n")
									assert.Contains(t, s, "A:x86_64\n")
									assert.Contains(t, s, "T:Gitea Test Package\n")
									assert.Contains(t, s, "U:https://gitea.io/\n")
									assert.Contains(t, s, "L:MIT\n")
									assert.Contains(t, s, "S:1353\n")
									assert.Contains(t, s, "I:4096\n")
									assert.Contains(t, s, "o:gitea-test\n")
									assert.Contains(t, s, "m:KN4CK3R <kn4ck3r@gitea.io>\n")
									assert.Contains(t, s, "t:1679498030\n")

									return true
								}
							}

							err = gzr.Reset(br)
							if err == io.EOF {
								break
							}
							assert.NoError(t, err)
						}

						return false
					})
				})

				t.Run("Download", func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s/x86_64/%s-%s.apk", rootURL, branch, repository, packageName, packageVersion))
					MakeRequest(t, req, http.StatusOK)
				})
			})
		}
	}

	t.Run("Delete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		for _, branch := range branches {
			for _, repository := range repositories {
				req := NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s/x86_64/%s-%s.apk", rootURL, branch, repository, packageName, packageVersion))
				MakeRequest(t, req, http.StatusUnauthorized)

				req = NewRequest(t, "DELETE", fmt.Sprintf("%s/%s/%s/x86_64/%s-%s.apk", rootURL, branch, repository, packageName, packageVersion))
				AddBasicAuthHeader(req, user.Name)
				MakeRequest(t, req, http.StatusNoContent)

				// Deleting the last file of an architecture should remove that index
				req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s/x86_64/APKINDEX.tar.gz", rootURL, branch, repository))
				MakeRequest(t, req, http.StatusNotFound)
			}
		}
	})
}

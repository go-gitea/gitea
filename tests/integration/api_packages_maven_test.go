// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/packages/maven"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageMaven(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	groupID := "com.gitea"
	artifactID := "test-project"
	packageVersion := "1.0.1"
	packageDescription := "Test Description"

	root := "/api/packages/user2/maven/com/gitea/test-project"
	filename := "any-name.jar"

	putFile := func(t *testing.T, path, content string, expectedStatus int) {
		req := NewRequestWithBody(t, "PUT", root+path, strings.NewReader(content)).AddBasicAuth(user.Name)
		MakeRequest(t, req, expectedStatus)
	}

	checkHeaders := func(t *testing.T, h http.Header, contentType string, contentLength int64) {
		assert.Equal(t, contentType, h.Get("Content-Type"))
		assert.Equal(t, strconv.FormatInt(contentLength, 10), h.Get("Content-Length"))
		assert.NotEmpty(t, h.Get("Last-Modified"))
	}

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		putFile(t, fmt.Sprintf("/%s/%s", packageVersion, filename), "test", http.StatusCreated)
		putFile(t, fmt.Sprintf("/%s/%s", packageVersion, filename), "test", http.StatusConflict)
		putFile(t, "/maven-metadata.xml", "test", http.StatusOK)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeMaven)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		require.NoError(t, err)
		assert.Nil(t, pd.SemVer)
		assert.Nil(t, pd.Metadata)
		assert.Equal(t, groupID+":"+artifactID, pd.Package.Name)
		assert.Equal(t, packageVersion, pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		require.NoError(t, err)
		assert.Len(t, pfs, 1)
		assert.Equal(t, filename, pfs[0].Name)
		assert.False(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		require.NoError(t, err)
		assert.Equal(t, int64(4), pb.Size)
	})

	t.Run("UploadLegacy", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// try to upload a package with legacy package name (will be saved as "GroupID-ArtifactID")
		legacyRootLink := "/api/packages/user2/maven/com/gitea/legacy-project"
		req := NewRequestWithBody(t, "PUT", legacyRootLink+"/1.0.2/any-file-name?use_legacy_package_name=1", strings.NewReader("test-content")).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		p, err := packages.GetPackageByName(db.DefaultContext, user.ID, packages.TypeMaven, "com.gitea-legacy-project")
		require.NoError(t, err)
		assert.Equal(t, "com.gitea-legacy-project", p.Name)

		req = NewRequest(t, "HEAD", legacyRootLink+"/1.0.2/any-file-name").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", "/user2/-/packages/maven/com.gitea-legacy-project/1.0.2")
		MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", "/user2/-/packages/maven/com.gitea:legacy-project/1.0.2")
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "GET", "/user2/-/packages/maven/com.gitea%3Alegacy-project/1.0.2")
		MakeRequest(t, req, http.StatusNotFound)

		// legacy package names should also be able to be listed
		req = NewRequest(t, "GET", legacyRootLink+"/maven-metadata.xml").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)
		respBody := resp.Body.String()
		assert.Contains(t, respBody, "<version>1.0.2</version>")

		// then upload a package with correct package name (will be saved as "GroupID:ArtifactID")
		req = NewRequestWithBody(t, "PUT", legacyRootLink+"/1.0.3/any-file-name", strings.NewReader("test-content")).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		_, err = packages.GetPackageByName(db.DefaultContext, user.ID, packages.TypeMaven, "com.gitea-legacy-project")
		require.ErrorIs(t, err, packages.ErrPackageNotExist)
		p, err = packages.GetPackageByName(db.DefaultContext, user.ID, packages.TypeMaven, "com.gitea:legacy-project")
		require.NoError(t, err)
		assert.Equal(t, "com.gitea:legacy-project", p.Name)
		req = NewRequest(t, "HEAD", legacyRootLink+"/1.0.2/any-file-name").AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", "/user2/-/packages/maven/com.gitea-legacy-project/1.0.2")
		MakeRequest(t, req, http.StatusNotFound)
		req = NewRequest(t, "GET", "/user2/-/packages/maven/com.gitea:legacy-project/1.0.2")
		MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", "/user2/-/packages/maven/com.gitea%3Alegacy-project/1.0.2")
		MakeRequest(t, req, http.StatusOK)

		// now 2 packages should be listed
		req = NewRequest(t, "GET", legacyRootLink+"/maven-metadata.xml").AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)
		respBody = resp.Body.String()
		assert.Contains(t, respBody, "<version>1.0.2</version>")
		assert.Contains(t, respBody, "<version>1.0.3</version>")
		require.NoError(t, packages.DeletePackageByID(db.DefaultContext, p.ID))
	})

	t.Run("UploadExists", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		putFile(t, fmt.Sprintf("/%s/%s", packageVersion, filename), "test", http.StatusConflict)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "HEAD", fmt.Sprintf("%s/%s/%s", root, packageVersion, filename)).AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		checkHeaders(t, resp.Header(), "application/java-archive", 4)

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s", root, packageVersion, filename)).AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		checkHeaders(t, resp.Header(), "application/java-archive", 4)

		assert.Equal(t, []byte("test"), resp.Body.Bytes())

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeMaven)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(0), pvs[0].DownloadCount)
	})

	t.Run("UploadVerifySHA1", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		t.Run("Missmatch", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			putFile(t, fmt.Sprintf("/%s/%s.sha1", packageVersion, filename), "test", http.StatusBadRequest)
		})
		t.Run("Valid", func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			putFile(t, fmt.Sprintf("/%s/%s.sha1", packageVersion, filename), "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3", http.StatusOK)
		})
	})

	pomContent := `<?xml version="1.0"?>
<project xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <groupId>` + groupID + `</groupId>
  <artifactId>` + artifactID + `</artifactId>
  <version>` + packageVersion + `</version>
  <description>` + packageDescription + `</description>
</project>`

	t.Run("UploadPOM", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeMaven)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		require.NoError(t, err)
		assert.Nil(t, pd.Metadata)

		putFile(t, fmt.Sprintf("/%s/%s.pom", packageVersion, filename), pomContent, http.StatusCreated)

		pvs, err = packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeMaven)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err = packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		require.NoError(t, err)
		assert.IsType(t, &maven.Metadata{}, pd.Metadata)
		assert.Equal(t, packageDescription, pd.Metadata.(*maven.Metadata).Description)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		require.NoError(t, err)
		assert.Len(t, pfs, 2)
		for _, pf := range pfs {
			if strings.HasSuffix(pf.Name, ".pom") {
				assert.Equal(t, filename+".pom", pf.Name)
				assert.True(t, pf.IsLead)
			} else {
				assert.False(t, pf.IsLead)
			}
		}
	})

	t.Run("DownloadPOM", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "HEAD", fmt.Sprintf("%s/%s/%s.pom", root, packageVersion, filename)).AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		checkHeaders(t, resp.Header(), "text/xml", int64(len(pomContent)))

		req = NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s.pom", root, packageVersion, filename)).AddBasicAuth(user.Name)
		resp = MakeRequest(t, req, http.StatusOK)

		checkHeaders(t, resp.Header(), "text/xml", int64(len(pomContent)))

		assert.Equal(t, []byte(pomContent), resp.Body.Bytes())

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeMaven)
		require.NoError(t, err)
		assert.Len(t, pvs, 1)
		assert.Equal(t, int64(1), pvs[0].DownloadCount)
	})

	t.Run("DownloadChecksums", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("%s/1.2.3/%s", root, filename)).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)

		for key, checksum := range map[string]string{
			"md5":    "098f6bcd4621d373cade4e832627b4f6",
			"sha1":   "a94a8fe5ccb19ba61c4c0873d391e987982fbbd3",
			"sha256": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			"sha512": "ee26b0dd4af7e749aa1a8ee3c10ae9923f618980772e473f8819a5d4940e0db27ac185f8a0e1d5f84f88bc887fd67b143732c304cc5fa9ad8e6f57f50028a8ff",
		} {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/%s/%s.%s", root, packageVersion, filename, key)).AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, checksum, resp.Body.String())
		}
	})

	t.Run("DownloadMetadata", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", root+"/maven-metadata.xml").AddBasicAuth(user.Name)
		resp := MakeRequest(t, req, http.StatusOK)

		expectedMetadata := `<?xml version="1.0" encoding="UTF-8"?>` + "\n<metadata><groupId>com.gitea</groupId><artifactId>test-project</artifactId><versioning><release>1.0.1</release><latest>1.0.1</latest><versions><version>1.0.1</version></versions></versioning></metadata>"

		checkHeaders(t, resp.Header(), "text/xml", int64(len(expectedMetadata)))

		assert.Equal(t, expectedMetadata, resp.Body.String())

		for key, checksum := range map[string]string{
			"md5":    "6bee0cebaaa686d658adf3e7e16371a0",
			"sha1":   "8696abce499fe84d9ea93e5492abe7147e195b6c",
			"sha256": "3f48322f81c4b2c3bb8649ae1e5c9801476162b520e1c2734ac06b2c06143208",
			"sha512": "cb075aa2e2ef1a83cdc14dd1e08c505b72d633399b39e73a21f00f0deecb39a3e2c79f157c1163f8a3854828750706e0dec3a0f5e4778e91f8ec2cf351a855f2",
		} {
			req := NewRequest(t, "GET", fmt.Sprintf("%s/maven-metadata.xml.%s", root, key)).AddBasicAuth(user.Name)
			resp := MakeRequest(t, req, http.StatusOK)

			assert.Equal(t, checksum, resp.Body.String())
		}
	})

	t.Run("UploadSnapshot", func(t *testing.T) {
		snapshotVersion := packageVersion + "-SNAPSHOT"

		putFile(t, fmt.Sprintf("/%s/%s", snapshotVersion, filename), "test", http.StatusCreated)
		putFile(t, "/maven-metadata.xml", "test", http.StatusOK)
		putFile(t, fmt.Sprintf("/%s/maven-metadata.xml", snapshotVersion), "test", http.StatusCreated)
		putFile(t, fmt.Sprintf("/%s/maven-metadata.xml", snapshotVersion), "test-overwrite", http.StatusCreated)
	})

	t.Run("InvalidFile", func(t *testing.T) {
		invalidVersion := packageVersion + "-invalid"
		putFile(t, fmt.Sprintf("/%s/%s", invalidVersion, filename), "any invalid content", http.StatusCreated)

		req := NewRequestf(t, "GET", "/%s/-/packages/maven/%s/%s", user.Name, url.QueryEscape(groupID+":"+artifactID), invalidVersion)
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), "No metadata.")
		assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))
	})
}

func TestPackageMavenConcurrent(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	groupID := "com.gitea"
	artifactID := "test-project"
	packageVersion := "1.0.1"

	root := fmt.Sprintf("/api/packages/%s/maven/%s/%s", user.Name, strings.ReplaceAll(groupID, ".", "/"), artifactID)

	putFile := func(t *testing.T, path, content string, expectedStatus int) {
		req := NewRequestWithBody(t, "PUT", root+path, strings.NewReader(content)).AddBasicAuth(user.Name)
		MakeRequest(t, req, expectedStatus)
	}

	t.Run("Concurrent Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				putFile(t, fmt.Sprintf("/%s/%s.jar", packageVersion, strconv.Itoa(i)), "test", http.StatusCreated)
				wg.Done()
			}(i)
		}
		wg.Wait()
	})
}

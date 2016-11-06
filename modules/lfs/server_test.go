package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetAuthed(t *testing.T) {
	req, err := http.NewRequest("GET", lfsServer.URL+"/user/repo/objects/"+contentOid, nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept", contentMediaType)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 200 {
		t.Fatalf("expected status 302, got %d", res.StatusCode)
	}

	by, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("expected response to contain content, got error: %s", err)
	}

	if string(by) != content {
		t.Fatalf("expected content to be `content`, got: %s", string(by))
	}
}

func TestGetAuthedWithRange(t *testing.T) {
	req, err := http.NewRequest("GET", lfsServer.URL+"/user/repo/objects/"+contentOid, nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept", contentMediaType)
	fromByte := 5
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", fromByte))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 206 {
		t.Fatalf("expected status 206, got %d", res.StatusCode)
	}
	if cr := res.Header.Get("Content-Range"); len(cr) > 0 {
		expected := fmt.Sprintf("bytes %d-%d/%d", fromByte, len(content)-1, len(content)-fromByte)
		if cr != expected {
			t.Fatalf("expected Content-Range header of %q, got %q", expected, cr)
		}
	} else {
		t.Fatalf("missing Content-Range header in response")
	}

	by, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("expected response to contain content, got error: %s", err)
	}

	if string(by) != content[fromByte:] {
		t.Fatalf("expected content to be `content`, got: %s", string(by))
	}
}

func TestGetUnauthed(t *testing.T) {
	req, err := http.NewRequest("GET", lfsServer.URL+"/user/repo/objects/"+contentOid, nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.Header.Set("Accept", contentMediaType)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 401 {
		t.Fatalf("expected status 401, got %d %s", res.StatusCode, req.URL)
	}
}

func TestGetMetaAuthed(t *testing.T) {
	req, err := http.NewRequest("GET", lfsServer.URL+"/bilbo/repo/objects/"+contentOid, nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept", metaMediaType)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d %s", res.StatusCode, req.URL)
	}

	var meta Representation
	dec := json.NewDecoder(res.Body)
	dec.Decode(&meta)

	if meta.Oid != contentOid {
		t.Fatalf("expected to see oid `%s` in meta, got: `%s`", contentOid, meta.Oid)
	}

	if meta.Size != contentSize {
		t.Fatalf("expected to see a size of `%d`, got: `%d`", contentSize, meta.Size)
	}

	download := meta.Actions["download"]
	if download.Href != "http://localhost:8080/bilbo/repo/objects/"+contentOid {
		t.Fatalf("expected download link, got %s", download.Href)
	}
}

func TestGetMetaUnauthed(t *testing.T) {
	req, err := http.NewRequest("GET", lfsServer.URL+"/user/repo/objects/"+contentOid, nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.Header.Set("Accept", metaMediaType)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 401 {
		t.Fatalf("expected status 401, got %d", res.StatusCode)
	}
}

func TestPostAuthedNewObject(t *testing.T) {
	req, err := http.NewRequest("POST", lfsServer.URL+"/bilbo/repo/objects", nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept", metaMediaType)

	buf := bytes.NewBufferString(fmt.Sprintf(`{"oid":"%s", "size":1234}`, nonexistingOid))
	req.Body = ioutil.NopCloser(buf)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 202 {
		t.Fatalf("expected status 202, got %d", res.StatusCode)
	}

	var meta Representation
	dec := json.NewDecoder(res.Body)
	dec.Decode(&meta)

	if meta.Oid != nonexistingOid {
		t.Fatalf("expected to see oid `%s` in meta, got: `%s`", nonexistingOid, meta.Oid)
	}

	if meta.Size != 1234 {
		t.Fatalf("expected to see a size of `1234`, got: `%d`", meta.Size)
	}

	if download, ok := meta.Actions["download"]; ok {
		t.Fatalf("expected POST to not contain a download link, got %s", download.Href)
	}

	upload, ok := meta.Actions["upload"]
	if !ok {
		t.Fatal("expected upload link to be present")
	}

	if upload.Href != "http://localhost:8080/bilbo/repo/objects/"+nonexistingOid {
		t.Fatalf("expected upload link, got %s", upload.Href)
	}
}

func TestPostAuthedExistingObject(t *testing.T) {
	req, err := http.NewRequest("POST", lfsServer.URL+"/bilbo/repo/objects", nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept", metaMediaType)

	buf := bytes.NewBufferString(fmt.Sprintf(`{"oid":"%s", "size":%d}`, contentOid, contentSize))
	req.Body = ioutil.NopCloser(buf)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	var meta Representation
	dec := json.NewDecoder(res.Body)
	dec.Decode(&meta)

	if meta.Oid != contentOid {
		t.Fatalf("expected to see oid `%s` in meta, got: `%s`", contentOid, meta.Oid)
	}

	if meta.Size != contentSize {
		t.Fatalf("expected to see a size of `%d`, got: `%d`", contentSize, meta.Size)
	}

	download := meta.Actions["download"]
	if download.Href != "http://localhost:8080/bilbo/repo/objects/"+contentOid {
		t.Fatalf("expected download link, got %s", download.Href)
	}

	upload, ok := meta.Actions["upload"]
	if !ok {
		t.Fatalf("expected upload link to be present")
	}

	if upload.Href != "http://localhost:8080/bilbo/repo/objects/"+contentOid {
		t.Fatalf("expected upload link, got %s", upload.Href)
	}
}

func TestPostUnauthed(t *testing.T) {
	req, err := http.NewRequest("POST", lfsServer.URL+"/bilbo/readonly/objects", nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.Header.Set("Accept", metaMediaType)

	buf := bytes.NewBufferString(fmt.Sprintf(`{"oid":"%s", "size":%d}`, contentOid, contentSize))
	req.Body = ioutil.NopCloser(buf)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 401 {
		t.Fatalf("expected status 401, got %d", res.StatusCode)
	}
}

func TestPut(t *testing.T) {
	req, err := http.NewRequest("PUT", lfsServer.URL+"/user/repo/objects/"+contentOid, nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept", contentMediaType)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(content)))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	r, err := testContentStore.Get(&MetaObject{Oid: contentOid}, 0)
	if err != nil {
		t.Fatalf("error retreiving from content store: %s", err)
	}
	c, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("error reading content: %s", err)
	}
	if string(c) != content {
		t.Fatalf("expected content, got `%s`", string(c))
	}
}

func TestMediaTypesRequired(t *testing.T) {
	m := []string{"GET", "PUT", "POST", "HEAD"}
	for _, method := range m {
		req, err := http.NewRequest(method, lfsServer.URL+"/user/repo/objects/"+contentOid, nil)
		if err != nil {
			t.Fatalf("request error: %s", err)
		}
		req.SetBasicAuth(testUser, testPass)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("response error: %s", err)
		}

		if res.StatusCode != 404 {
			t.Fatalf("expected status 404, got %d", res.StatusCode)
		}
	}
}

func TestMediaTypesParsed(t *testing.T) {
	req, err := http.NewRequest("GET", lfsServer.URL+"/user/repo/objects/"+contentOid, nil)
	if err != nil {
		t.Fatalf("request error: %s", err)
	}
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept", contentMediaType+"; charset=utf-8")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("response error: %s", err)
	}

	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
}

var (
	lfsServer        *httptest.Server
	testMetaStore    *MetaStore
	testContentStore *ContentStore
	testUser         = "bilbo"
	testPass         = "baggins"
	testAuth         = fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(testUser+":"+testPass)))
	badAuth          = fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte("azog:defiler")))
	content          = "this is my content"
	contentSize      = int64(len(content))
	contentOid       = "f97e1b2936a56511b3b6efc99011758e4700d60fb1674d31445d1ee40b663f24"
	nonexistingOid   = "aec070645fe53ee3b3763059376134f058cc337247c978add178b6ccdfb0019f"
)

func TestMain(m *testing.M) {
	os.Remove("lfs-test.db")

	var err error
	testMetaStore, err = NewMetaStore("lfs-test.db")
	if err != nil {
		fmt.Printf("Error creating meta store: %s", err)
		os.Exit(1)
	}

	testContentStore, err = NewContentStore("lfs-content-test")
	if err != nil {
		fmt.Printf("Error creating content store: %s", err)
		os.Exit(1)
	}

	if err := seedMetaStore(); err != nil {
		fmt.Printf("Error seeding meta store: %s", err)
		os.Exit(1)
	}

	if err := seedContentStore(); err != nil {
		fmt.Printf("Error seeding content store: %s", err)
		os.Exit(1)
	}

	app := NewApp(testContentStore, testMetaStore)
	lfsServer = httptest.NewServer(app)

	logger = NewKVLogger(ioutil.Discard)

	ret := m.Run()

	lfsServer.Close()
	testMetaStore.Close()
	os.Remove("lfs-test.db")
	os.RemoveAll("lfs-content-test")

	os.Exit(ret)
}

func seedMetaStore() error {
	if err := testMetaStore.AddUser(testUser, testPass); err != nil {
		return err
	}

	rv := &RequestVars{Authorization: testAuth, Oid: contentOid, Size: contentSize}
	if _, err := testMetaStore.Put(rv); err != nil {
		return err
	}

	return nil
}

func seedContentStore() error {
	meta := &MetaObject{Oid: contentOid, Size: contentSize}
	buf := bytes.NewBuffer([]byte(content))
	if err := testContentStore.Put(meta, buf); err != nil {
		return err
	}

	return nil
}

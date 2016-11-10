package testutil

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"

	"github.com/go-gitea/gitea/cmd"
	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/setting"
	"github.com/go-gitea/gitea/routers"

	"gopkg.in/macaron.v1"
	"gopkg.in/testfixtures.v2"
)

const (
	CONTENT_TYPE_FORM = "application/x-www-form-urlencoded"
	CONTENT_TYPE_JSON = "application/json"
)

var (
	theMacaron *macaron.Macaron
	fixtures   *testfixtures.Context
)

func getMacaron() *macaron.Macaron {
	if theMacaron == nil {
		theMacaron = cmd.NewMacaron()
	}
	return theMacaron
}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	getMacaron().ServeHTTP(w, r)
}

func NewTestContext(method, path, contentType string, body []byte, userId string) (w *httptest.ResponseRecorder, r *http.Request) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	w = httptest.NewRecorder()
	r, _ = http.NewRequest(method, path, bodyReader)
	if len(contentType) > 0 {
		r.Header.Set("Content-Type", contentType)
		if contentType == CONTENT_TYPE_FORM {
			r.PostForm = url.Values{}
		}
	}
	if len(userId) > 0 {
		r.AddCookie(&http.Cookie{Name: "user_id", Value: userId})
	}
	return
}

func TableCount(tableName string) (count int64) {
	models.Database().QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", tableName)).Scan(&count)
	return
}

func LastId(tableName string) (lastId int64) {
	models.Database().QueryRow(fmt.Sprintf("SELECT MAX(id) FROM \"%s\"", tableName)).Scan(&lastId)
	return
}

func TestGlobalInit() {
	setting.CustomConf = filepath.Join(setting.GiteaPath(), "testdata/app_test.ini")
	routers.GlobalInit()

	var err error
	fixtures, err = testfixtures.NewFolder(
		models.Database(),
		&testfixtures.SQLite{},
		filepath.Join(setting.GiteaPath(), "testdata/fixtures"),
	)
	if err != nil {
		log.Fatal(err)
	}
}

func PrepareTestDatabase() {
	if err := fixtures.Load(); err != nil {
		log.Fatal(err)
	}
}

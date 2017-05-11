// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/routes"

	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
	"gopkg.in/macaron.v1"
	"gopkg.in/testfixtures.v2"
)

var mac *macaron.Macaron

func TestMain(m *testing.M) {
	initIntegrationTest()
	mac = routes.NewMacaron()
	routes.RegisterRoutes(mac)

	var helper testfixtures.Helper
	if setting.UseMySQL {
		helper = &testfixtures.MySQL{}
	} else if setting.UsePostgreSQL {
		helper = &testfixtures.PostgreSQL{}
	} else if setting.UseSQLite3 {
		helper = &testfixtures.SQLite{}
	} else {
		fmt.Println("Unsupported RDBMS for integration tests")
		os.Exit(1)
	}

	err := models.InitFixtures(
		helper,
		"models/fixtures/",
	)
	if err != nil {
		fmt.Printf("Error initializing test database: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func initIntegrationTest() {
	if setting.CustomConf = os.Getenv("GITEA_CONF"); setting.CustomConf == "" {
		fmt.Println("Environment variable $GITEA_CONF not set")
		os.Exit(1)
	}
	if os.Getenv("GITEA_ROOT") == "" {
		fmt.Println("Environment variable $GITEA_ROOT not set")
		os.Exit(1)
	}

	setting.NewContext()
	models.LoadConfigs()

	switch {
	case setting.UseMySQL:
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/",
			models.DbCfg.User, models.DbCfg.Passwd, models.DbCfg.Host))
		defer db.Close()
		if err != nil {
			log.Fatalf("sql.Open: %v", err)
		}
		if _, err = db.Exec("CREATE DATABASE IF NOT EXISTS testgitea"); err != nil {
			log.Fatalf("db.Exec: %v", err)
		}
	case setting.UsePostgreSQL:
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
			models.DbCfg.User, models.DbCfg.Passwd, models.DbCfg.Host, models.DbCfg.SSLMode))
		defer db.Close()
		if err != nil {
			log.Fatalf("sql.Open: %v", err)
		}
		rows, err := db.Query(fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'",
			models.DbCfg.Name))
		if err != nil {
			log.Fatalf("db.Query: %v", err)
		}
		defer rows.Close()

		if rows.Next() {
			break
		}
		if _, err = db.Exec("CREATE DATABASE testgitea"); err != nil {
			log.Fatalf("db.Exec: %v", err)
		}
	}
	routers.GlobalInit()
}

func prepareTestEnv(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())
	assert.NoError(t, os.RemoveAll("integrations/gitea-integration"))
	assert.NoError(t, com.CopyDir("integrations/gitea-integration-meta", "integrations/gitea-integration"))
}

type TestSession struct {
	jar http.CookieJar
}

func (s *TestSession) GetCookie(name string) *http.Cookie {
	baseURL, err := url.Parse(setting.AppURL)
	if err != nil {
		return nil
	}

	for _, c := range s.jar.Cookies(baseURL) {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (s *TestSession) MakeRequest(t *testing.T, req *http.Request) *TestResponse {
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	for _, c := range s.jar.Cookies(baseURL) {
		req.AddCookie(c)
	}
	resp := MakeRequest(req)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Headers["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}
	s.jar.SetCookies(baseURL, cr.Cookies())

	return resp
}

func loginUser(t *testing.T, userName, password string) *TestSession {
	req, err := http.NewRequest("GET", "/user/login", nil)
	assert.NoError(t, err)
	resp := MakeRequest(req)
	assert.EqualValues(t, http.StatusOK, resp.HeaderCode)

	doc, err := NewHtmlParser(resp.Body)
	assert.NoError(t, err)

	req, err = http.NewRequest("POST", "/user/login",
		bytes.NewBufferString(url.Values{
			"_csrf":     []string{doc.GetInputValueByName("_csrf")},
			"user_name": []string{userName},
			"password":  []string{password},
		}.Encode()),
	)
	assert.NoError(t, err)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp = MakeRequest(req)
	assert.EqualValues(t, http.StatusFound, resp.HeaderCode)

	ch := http.Header{}
	ch.Add("Cookie", strings.Join(resp.Headers["Set-Cookie"], ";"))
	cr := http.Request{Header: ch}

	jar, err := cookiejar.New(nil)
	assert.NoError(t, err)
	baseURL, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	jar.SetCookies(baseURL, cr.Cookies())

	return &TestSession{jar: jar}
}

type TestResponseWriter struct {
	HeaderCode int
	Writer     io.Writer
	Headers    http.Header
}

func (w *TestResponseWriter) Header() http.Header {
	return w.Headers
}

func (w *TestResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *TestResponseWriter) WriteHeader(n int) {
	w.HeaderCode = n
}

type TestResponse struct {
	HeaderCode int
	Body       []byte
	Headers    http.Header
}

func MakeRequest(req *http.Request) *TestResponse {
	buffer := bytes.NewBuffer(nil)
	respWriter := &TestResponseWriter{
		Writer:  buffer,
		Headers: make(map[string][]string),
	}
	mac.ServeHTTP(respWriter, req)
	return &TestResponse{
		HeaderCode: respWriter.HeaderCode,
		Body:       buffer.Bytes(),
		Headers:    respWriter.Headers,
	}
}

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
	"os"
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
		if rows.Next() {
			break // database already exists
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

type TestResponseWriter struct {
	HeaderCode int
	Writer     io.Writer
}

func (w *TestResponseWriter) Header() http.Header {
	return make(map[string][]string)
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
}

func MakeRequest(req *http.Request) *TestResponse {
	buffer := bytes.NewBuffer(nil)
	respWriter := &TestResponseWriter{
		Writer: buffer,
	}
	mac.ServeHTTP(respWriter, req)
	return &TestResponse{
		HeaderCode: respWriter.HeaderCode,
		Body:       buffer.Bytes(),
	}
}

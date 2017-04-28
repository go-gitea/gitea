// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/routes"

	"gopkg.in/macaron.v1"
	"gopkg.in/testfixtures.v2"
)

var mac *macaron.Macaron

func TestMain(m *testing.M) {
	appIniPath := os.Getenv("GITEA_CONF")
	if appIniPath == "" {
		fmt.Println("Environment variable $GITEA_CONF not set")
		os.Exit(1)
	}
	setting.CustomConf = appIniPath
	routers.GlobalInit()
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
		"integrations/gitea-integration/fixtures/",
	)
	if err != nil {
		fmt.Printf("Error initializing test database: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
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

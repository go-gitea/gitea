package common_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code.gitea.io/gitea/routers/common"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestMetricsMiddlewere(t *testing.T) {

	middleware := common.RouteMetrics()
	r := chi.NewRouter()
	r.Use(middleware)
	r.Get("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
		time.Sleep(5 * time.Millisecond)
	}))

	testServer := httptest.NewServer(r)

	_, err := http.Get(testServer.URL)
	require.NoError(t, err)

}

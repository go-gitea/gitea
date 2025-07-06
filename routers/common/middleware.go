// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/gtprof"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/routing"
	"code.gitea.io/gitea/services/context"

	"gitea.com/go-chi/session"
	"github.com/chi-middleware/proxy"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	httpRequestMethod      = "http_request_method"
	httpResponseStatusCode = "http_response_status_code"
	httpRoute              = "http_route"
	kb                     = 1000
	mb                     = kb * kb
)

// reference: https://opentelemetry.io/docs/specs/semconv/http/http-metrics/#http-server
var (
	sizeBuckets = []float64{1 * kb, 2 * kb, 5 * kb, 10 * kb, 100 * kb, 500 * kb, 1 * mb, 2 * mb, 5 * mb, 10 * mb}
	// reqInflightGauge tracks the amount of currently handled requests
	reqInflightGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "http",
		Subsystem: "server",
		Name:      "active_requests",
		Help:      "Number of active HTTP server requests.",
	}, []string{httpRequestMethod})
	// reqDurationHistogram tracks the time taken by http request
	reqDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "http",
		Subsystem: "server",
		Name:      "request_duration_seconds", // diverge from spec to store the unit in metric.
		Help:      "Measures the latency of HTTP requests processed by the server",
		Buckets:   []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30, 60, 120, 300}, // based on dotnet buckets https://github.com/open-telemetry/semantic-conventions/issues/336
	}, []string{httpRequestMethod, httpResponseStatusCode, httpRoute})
	// reqSizeHistogram tracks the size of request
	reqSizeHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "http",
		Subsystem: "server_request",
		Name:      "body_size",
		Help:      "Size of HTTP server request bodies.",
		Buckets:   sizeBuckets,
	}, []string{httpRequestMethod, httpResponseStatusCode, httpRoute})
	// respSizeHistogram tracks the size of the response
	respSizeHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "http",
		Subsystem: "server_response",
		Name:      "body_size",
		Help:      "Size of HTTP server response bodies.",
		Buckets:   sizeBuckets,
	}, []string{httpRequestMethod, httpResponseStatusCode, httpRoute})
)

// ProtocolMiddlewares returns HTTP protocol related middlewares, and it provides a global panic recovery
func ProtocolMiddlewares() (handlers []any) {
	// the order is important
	handlers = append(handlers, ChiRoutePathHandler())   // make sure chi has correct paths
	handlers = append(handlers, RequestContextHandler()) //	prepare the context and panic recovery

	if setting.ReverseProxyLimit > 0 && len(setting.ReverseProxyTrustedProxies) > 0 {
		handlers = append(handlers, ForwardedHeadersHandler(setting.ReverseProxyLimit, setting.ReverseProxyTrustedProxies))
	}

	if setting.IsRouteLogEnabled() {
		handlers = append(handlers, routing.NewLoggerHandler())
	}

	if setting.IsAccessLogEnabled() {
		handlers = append(handlers, context.AccessLogger())
	}
	if setting.Metrics.Enabled {
		handlers = append(handlers, RouteMetrics())
	}

	return handlers
}

func RequestContextHandler() func(h http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(respOrig http.ResponseWriter, req *http.Request) {
			// this response writer might not be the same as the one in context.Base.Resp
			// because there might be a "gzip writer" in the middle, so the "written size" here is the compressed size
			respWriter := context.WrapResponseWriter(respOrig)

			profDesc := fmt.Sprintf("HTTP: %s %s", req.Method, req.RequestURI)
			ctx, finished := reqctx.NewRequestContext(req.Context(), profDesc)
			defer finished()

			ctx, span := gtprof.GetTracer().Start(ctx, gtprof.TraceSpanHTTP)
			req = req.WithContext(ctx)
			defer func() {
				chiCtx := chi.RouteContext(req.Context())
				span.SetAttributeString(gtprof.TraceAttrHTTPRoute, chiCtx.RoutePattern())
				span.End()
			}()

			defer func() {
				if err := recover(); err != nil {
					RenderPanicErrorPage(respWriter, req, err) // it should never panic
				}
			}()

			ds := reqctx.GetRequestDataStore(ctx)
			req = req.WithContext(cache.WithCacheContext(ctx))
			ds.SetContextValue(httplib.RequestContextKey, req)
			ds.AddCleanUp(func() {
				if req.MultipartForm != nil {
					_ = req.MultipartForm.RemoveAll() // remove the temp files buffered to tmp directory
				}
			})
			next.ServeHTTP(respWriter, req)
		})
	}
}

func ChiRoutePathHandler() func(h http.Handler) http.Handler {
	// make sure chi uses EscapedPath(RawPath) as RoutePath, then "%2f" could be handled correctly
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			chiCtx := chi.RouteContext(req.Context())
			if req.URL.RawPath == "" {
				chiCtx.RoutePath = req.URL.EscapedPath()
			} else {
				chiCtx.RoutePath = req.URL.RawPath
			}
			next.ServeHTTP(resp, req)
		})
	}
}

func ForwardedHeadersHandler(limit int, trustedProxies []string) func(h http.Handler) http.Handler {
	opt := proxy.NewForwardedHeadersOptions().WithForwardLimit(limit).ClearTrustedProxies()
	for _, n := range trustedProxies {
		if !strings.Contains(n, "/") {
			opt.AddTrustedProxy(n)
		} else {
			opt.AddTrustedNetwork(n)
		}
	}
	return proxy.ForwardedHeaders(opt)
}

// RouteMetrics instruments http requests and responses
func RouteMetrics() func(h http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			inflight := reqInflightGauge.WithLabelValues(req.Method)
			inflight.Inc()
			defer inflight.Dec()
			start := time.Now()

			next.ServeHTTP(resp, req)
			m := context.WrapResponseWriter(resp)
			route := chi.RouteContext(req.Context()).RoutePattern()
			code := strconv.Itoa(m.WrittenStatus())
			reqDurationHistogram.WithLabelValues(req.Method, code, route).Observe(time.Since(start).Seconds())
			respSizeHistogram.WithLabelValues(req.Method, code, route).Observe(float64(m.WrittenSize()))
			size := req.ContentLength
			if size < 0 {
				size = 0
			}
			reqSizeHistogram.WithLabelValues(req.Method, code, route).Observe(float64(size))
		})
	}
}

func Sessioner() func(next http.Handler) http.Handler {
	return session.Sessioner(session.Options{
		Provider:       setting.SessionConfig.Provider,
		ProviderConfig: setting.SessionConfig.ProviderConfig,
		CookieName:     setting.SessionConfig.CookieName,
		CookiePath:     setting.SessionConfig.CookiePath,
		Gclifetime:     setting.SessionConfig.Gclifetime,
		Maxlifetime:    setting.SessionConfig.Maxlifetime,
		Secure:         setting.SessionConfig.Secure,
		SameSite:       setting.SessionConfig.SameSite,
		Domain:         setting.SessionConfig.Domain,
	})
}

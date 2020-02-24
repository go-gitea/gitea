package cors

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	macaron "gitea.com/macaron/macaron"
)

const version = "0.1.1"

const anyDomain = "!*"

// Version returns the version of this module
func Version() string {
	return version
}

/*
Options to configure the CORS middleware read from the [cors] section of the ini configuration file.

SCHEME may be http or https as accepted schemes or the '*' wildcard to accept any scheme.

ALLOW_DOMAIN may be a comma separated list of domains that are allowed to run CORS requests
Special values are the  a single '*' wildcard that will allow any domain to send requests without
credentials and the special '!*' wildcard which will reply with requesting domain in the 'access-control-allow-origin'
header and hence allow requess from any domain *with* credentials.

ALLOW_SUBDOMAIN set to true accepts requests from any subdomain of ALLOW_DOMAIN.

METHODS may be a comma separated list of HTTP-methods to be accepted.

MAX_AGE_SECONDS may be the duration in secs for which the response is cached (default 600).
ref: https://stackoverflow.com/questions/54300997/is-it-possible-to-cache-http-options-response?noredirect=1#comment95790277_54300997
ref: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age

ALLOW_CREDENTIALS set to false rejects any request with credentials.
*/
type Options struct {
	Section          string
	Scheme           string
	AllowDomain      []string
	AllowSubdomain   bool
	Methods          []string
	MaxAgeSeconds    int
	AllowCredentials bool
}

func prepareOptions(options []Options) Options {
	var opt Options
	if len(options) > 0 {
		opt = options[0]
	}

	if len(opt.Section) == 0 {
		opt.Section = "cors"
	}
	sec := macaron.Config().Section(opt.Section)

	if len(opt.Scheme) == 0 {
		opt.Scheme = sec.Key("SCHEME").MustString("http")
	}
	if len(opt.AllowDomain) == 0 {
		opt.AllowDomain = sec.Key("ALLOW_DOMAIN").Strings(",")
		if len(opt.AllowDomain) == 0 {
			opt.AllowDomain = []string{"*"}
		}
	}
	if !opt.AllowSubdomain {
		opt.AllowSubdomain = sec.Key("ALLOW_SUBDOMAIN").MustBool(false)
	}
	if len(opt.Methods) == 0 {
		opt.Methods = sec.Key("METHODS").Strings(",")
		if len(opt.Methods) == 0 {
			opt.Methods = []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
				http.MethodOptions,
			}
		}
	}
	if opt.MaxAgeSeconds <= 0 {
		opt.MaxAgeSeconds = sec.Key("MAX_AGE_SECONDS").MustInt(600)
	}
	if !opt.AllowCredentials {
		opt.AllowCredentials = sec.Key("ALLOW_CREDENTIALS").MustBool(true)
	}

	return opt
}

// CORS responds to preflight requests with adequat access-control-* respond headers
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin
// https://fetch.spec.whatwg.org/#cors-protocol-and-credentials
func CORS(options ...Options) macaron.Handler {
	opt := prepareOptions(options)
	return func(ctx *macaron.Context, log *log.Logger) {
		reqOptions := ctx.Req.Method == http.MethodOptions

		headers := map[string]string{
			"access-control-allow-methods": strings.Join(opt.Methods, ","),
			"access-control-allow-headers": ctx.Req.Header.Get("access-control-request-headers"),
			"access-control-max-age":       strconv.Itoa(opt.MaxAgeSeconds),
		}
		if opt.AllowDomain[0] == "*" {
			headers["access-control-allow-origin"] = "*"
		} else {
			origin := ctx.Req.Header.Get("Origin")
			if reqOptions && origin == "" {
				respErrorf(ctx, log, http.StatusBadRequest, "missing origin header in CORS request")
				return
			}

			u, err := url.Parse(origin)
			if err != nil {
				respErrorf(ctx, log, http.StatusBadRequest, "Failed to parse CORS origin header. Reason: %v", err)
				return
			}

			ok := false
			for _, d := range opt.AllowDomain {
				if u.Hostname() == d || (opt.AllowSubdomain && strings.HasSuffix(u.Hostname(), "."+d)) || d == anyDomain {
					ok = true
					break
				}
			}
			if ok {
				if opt.Scheme != "*" {
					u.Scheme = opt.Scheme
				}
				headers["access-control-allow-origin"] = u.String()
				headers["access-control-allow-credentials"] = strconv.FormatBool(opt.AllowCredentials)
				headers["vary"] = "Origin"
			}
			if reqOptions && !ok {
				respErrorf(ctx, log, http.StatusBadRequest, "CORS request from prohibited domain %v", origin)
				return
			}
		}
		ctx.Resp.Before(func(w macaron.ResponseWriter) {
			for k, v := range headers {
				w.Header().Set(k, v)
			}
		})
		if reqOptions {
			ctx.Status(200) // return response
		}
	}
}

func respErrorf(ctx *macaron.Context, log *log.Logger, statusCode int, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	log.Println(msg)
	ctx.WriteHeader(statusCode)
	_, err := ctx.Write([]byte(msg))
	if err != nil {
		panic(err)
	}
	return
}

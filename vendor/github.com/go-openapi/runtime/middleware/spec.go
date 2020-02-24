// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middleware

import (
	"net/http"
	"path"
)

// Spec creates a middleware to serve a swagger spec.
// This allows for altering the spec before starting the http listener.
// This can be useful if you want to serve the swagger spec from another path than /swagger.json
//
func Spec(basePath string, b []byte, next http.Handler) http.Handler {
	if basePath == "" {
		basePath = "/"
	}
	pth := path.Join(basePath, "swagger.json")

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pth {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusOK)
			//#nosec
			_, _ = rw.Write(b)
			return
		}

		if next == nil {
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		next.ServeHTTP(rw, r)
	})
}

// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"fmt"
	"net/http"
	"reflect"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web/routing"
	"code.gitea.io/gitea/modules/web/types"
)

var responseStatusProviders = map[reflect.Type]func(req *http.Request) types.ResponseStatusProvider{}

func RegisterResponseStatusProvider[T any](fn func(req *http.Request) types.ResponseStatusProvider) {
	responseStatusProviders[reflect.TypeOf((*T)(nil)).Elem()] = fn
}

// responseWriter is a wrapper of http.ResponseWriter, to check whether the response has been written
type responseWriter struct {
	respWriter http.ResponseWriter
	status     int
}

var _ types.ResponseStatusProvider = (*responseWriter)(nil)

func (r *responseWriter) WrittenStatus() int {
	return r.status
}

func (r *responseWriter) Header() http.Header {
	return r.respWriter.Header()
}

func (r *responseWriter) Write(bytes []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.respWriter.Write(bytes)
}

func (r *responseWriter) WriteHeader(statusCode int) {
	r.status = statusCode
	r.respWriter.WriteHeader(statusCode)
}

var (
	httpReqType    = reflect.TypeOf((*http.Request)(nil))
	respWriterType = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
)

// preCheckHandler checks whether the handler is valid, developers could get first-time feedback, all mistakes could be found at startup
func preCheckHandler(fn reflect.Value, argsIn []reflect.Value) {
	hasStatusProvider := false
	for _, argIn := range argsIn {
		if _, hasStatusProvider = argIn.Interface().(types.ResponseStatusProvider); hasStatusProvider {
			break
		}
	}
	if !hasStatusProvider {
		panic(fmt.Sprintf("handler should have at least one ResponseStatusProvider argument, but got %s", fn.Type()))
	}
	if fn.Type().NumOut() != 0 {
		panic(fmt.Sprintf("handler should have no return value other than registered ones, but got %s", fn.Type()))
	}
}

func prepareHandleArgsIn(resp http.ResponseWriter, req *http.Request, fn reflect.Value, fnInfo *routing.FuncInfo) []reflect.Value {
	defer func() {
		if err := recover(); err != nil {
			log.Error("unable to prepare handler arguments for %s: %v", fnInfo.String(), err)
			panic(err)
		}
	}()
	isPreCheck := req == nil

	argsIn := make([]reflect.Value, fn.Type().NumIn())
	for i := 0; i < fn.Type().NumIn(); i++ {
		argTyp := fn.Type().In(i)
		switch argTyp {
		case respWriterType:
			argsIn[i] = reflect.ValueOf(resp)
		case httpReqType:
			argsIn[i] = reflect.ValueOf(req)
		default:
			if argFn, ok := responseStatusProviders[argTyp]; ok {
				if isPreCheck {
					argsIn[i] = reflect.ValueOf(&responseWriter{})
				} else {
					argsIn[i] = reflect.ValueOf(argFn(req))
				}
			} else {
				panic(fmt.Sprintf("unsupported argument type: %s", argTyp))
			}
		}
	}
	return argsIn
}

func handleResponse(fn reflect.Value, ret []reflect.Value) {
	if len(ret) != 0 {
		panic(fmt.Sprintf("unsupported return values: %s", fn.Type()))
	}
}

func hasResponseBeenWritten(argsIn []reflect.Value) bool {
	for _, argIn := range argsIn {
		if statusProvider, ok := argIn.Interface().(types.ResponseStatusProvider); ok {
			if statusProvider.WrittenStatus() != 0 {
				return true
			}
		}
	}
	return false
}

func wrapHandlerProvider[T http.Handler](hp func(next http.Handler) T, funcInfo *routing.FuncInfo) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		h := hp(next) // this handle could be dynamically generated, so we can't use it for debug info
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			routing.UpdateFuncInfo(req.Context(), funcInfo)
			h.ServeHTTP(resp, req)
		})
	}
}

// toHandlerProvider converts a handler to a handler provider
// A handler provider is a function that takes a "next" http.Handler, it can be used as a middleware
func toHandlerProvider(handler any) func(next http.Handler) http.Handler {
	funcInfo := routing.GetFuncInfo(handler)
	fn := reflect.ValueOf(handler)
	if fn.Type().Kind() != reflect.Func {
		panic(fmt.Sprintf("handler must be a function, but got %s", fn.Type()))
	}

	if hp, ok := handler.(func(next http.Handler) http.Handler); ok {
		return wrapHandlerProvider(hp, funcInfo)
	} else if hp, ok := handler.(func(http.Handler) http.HandlerFunc); ok {
		return wrapHandlerProvider(hp, funcInfo)
	}

	provider := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(respOrig http.ResponseWriter, req *http.Request) {
			// wrap the response writer to check whether the response has been written
			resp := respOrig
			if _, ok := resp.(types.ResponseStatusProvider); !ok {
				resp = &responseWriter{respWriter: resp}
			}

			// prepare the arguments for the handler and do pre-check
			argsIn := prepareHandleArgsIn(resp, req, fn, funcInfo)
			if req == nil {
				preCheckHandler(fn, argsIn)
				return // it's doing pre-check, just return
			}

			routing.UpdateFuncInfo(req.Context(), funcInfo)
			ret := fn.Call(argsIn)

			// handle the return value (no-op at the moment)
			handleResponse(fn, ret)

			// if the response has not been written, call the next handler
			if next != nil && !hasResponseBeenWritten(argsIn) {
				next.ServeHTTP(resp, req)
			}
		})
	}

	provider(nil).ServeHTTP(nil, nil) // do a pre-check to make sure all arguments and return values are supported
	return provider
}

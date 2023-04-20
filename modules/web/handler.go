// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	goctx "context"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/routing"
)

// ResponseStatusProvider is an interface to check whether the response has been written by the handler
type ResponseStatusProvider interface {
	Written() bool
}

// TODO: decouple this from the context package, let the context package register these providers
var argTypeProvider = map[reflect.Type]func(req *http.Request) ResponseStatusProvider{
	reflect.TypeOf(&context.APIContext{}):     func(req *http.Request) ResponseStatusProvider { return context.GetAPIContext(req) },
	reflect.TypeOf(&context.Context{}):        func(req *http.Request) ResponseStatusProvider { return context.GetContext(req) },
	reflect.TypeOf(&context.PrivateContext{}): func(req *http.Request) ResponseStatusProvider { return context.GetPrivateContext(req) },
}

// responseWriter is a wrapper of http.ResponseWriter, to check whether the response has been written
type responseWriter struct {
	respWriter http.ResponseWriter
	status     int
}

var _ ResponseStatusProvider = (*responseWriter)(nil)

func (r *responseWriter) Written() bool {
	return r.status > 0
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
	cancelFuncType = reflect.TypeOf((*goctx.CancelFunc)(nil)).Elem()
)

// preCheckHandler checks whether the handler is valid, developers could get first-time feedback, all mistakes could be found at startup
func preCheckHandler(fn reflect.Value, argsIn []reflect.Value) {
	hasStatusProvider := false
	for _, argIn := range argsIn {
		if _, hasStatusProvider = argIn.Interface().(ResponseStatusProvider); hasStatusProvider {
			break
		}
	}
	if !hasStatusProvider {
		panic(fmt.Sprintf("handler should have at least one ResponseStatusProvider argument, but got %s", fn.Type()))
	}
	if fn.Type().NumOut() != 0 && fn.Type().NumIn() != 1 {
		panic(fmt.Sprintf("handler should have no return value or only one argument, but got %s", fn.Type()))
	}
	if fn.Type().NumOut() == 1 && fn.Type().Out(0) != cancelFuncType {
		panic(fmt.Sprintf("handler should return a cancel function, but got %s", fn.Type()))
	}
}

func prepareHandleArgsIn(resp http.ResponseWriter, req *http.Request, fn reflect.Value) []reflect.Value {
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
			if argFn, ok := argTypeProvider[argTyp]; ok {
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

func handleResponse(fn reflect.Value, ret []reflect.Value) goctx.CancelFunc {
	if len(ret) == 1 {
		if cancelFunc, ok := ret[0].Interface().(goctx.CancelFunc); ok {
			return cancelFunc
		}
		panic(fmt.Sprintf("unsupported return type: %s", ret[0].Type()))
	} else if len(ret) > 1 {
		panic(fmt.Sprintf("unsupported return values: %s", fn.Type()))
	}
	return nil
}

func hasResponseBeenWritten(argsIn []reflect.Value) bool {
	for _, argIn := range argsIn {
		if statusProvider, ok := argIn.Interface().(ResponseStatusProvider); ok {
			if statusProvider.Written() {
				return true
			}
		}
	}
	return false
}

// toHandlerProvider converts a handler to a handler provider
// A handler provider is a function that takes a "next" http.Handler, it can be used as a middleware
func toHandlerProvider(handler any) func(next http.Handler) http.Handler {
	if hp, ok := handler.(func(next http.Handler) http.Handler); ok {
		return hp
	}

	funcInfo := routing.GetFuncInfo(handler)
	fn := reflect.ValueOf(handler)
	if fn.Type().Kind() != reflect.Func {
		panic(fmt.Sprintf("handler must be a function, but got %s", fn.Type()))
	}

	provider := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(respOrig http.ResponseWriter, req *http.Request) {
			// wrap the response writer to check whether the response has been written
			resp := respOrig
			if _, ok := resp.(ResponseStatusProvider); !ok {
				resp = &responseWriter{respWriter: resp}
			}

			// prepare the arguments for the handler and do pre-check
			argsIn := prepareHandleArgsIn(resp, req, fn)
			if req == nil {
				preCheckHandler(fn, argsIn)
				return // it's doing pre-check, just return
			}

			routing.UpdateFuncInfo(req.Context(), funcInfo)
			ret := fn.Call(argsIn)

			// handle the return value, and defer the cancel function if there is one
			cancelFunc := handleResponse(fn, ret)
			if cancelFunc != nil {
				defer cancelFunc()
			}

			// if the response has not been written, call the next handler
			if next != nil && !hasResponseBeenWritten(argsIn) {
				next.ServeHTTP(resp, req)
			}
		})
	}

	provider(nil).ServeHTTP(nil, nil) // do a pre-check to make sure all arguments and return values are supported
	return provider
}

// MiddlewareWithPrefix wraps a handler function at a prefix, and make it as a middleware
// TODO: this design is incorrect, the asset handler should not be a middleware
func MiddlewareWithPrefix(pathPrefix string, middleware func(handler http.Handler) http.Handler, handlerFunc http.HandlerFunc) func(next http.Handler) http.Handler {
	funcInfo := routing.GetFuncInfo(handlerFunc)
	handler := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		routing.UpdateFuncInfo(req.Context(), funcInfo)
		handlerFunc(resp, req)
	})
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if !strings.HasPrefix(req.URL.Path, pathPrefix) {
				next.ServeHTTP(resp, req)
				return
			}
			if middleware != nil {
				middleware(handler).ServeHTTP(resp, req)
			} else {
				handler.ServeHTTP(resp, req)
			}
		})
	}
}

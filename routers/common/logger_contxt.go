// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"context"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
)

// LogRequestTrigger indicates when the logger is triggered
type LogRequestTrigger int

const (
	//LogRequestStart at the beginning of a request
	LogRequestStart LogRequestTrigger = iota

	//LogRequestExecuting the request is still executing
	LogRequestExecuting

	//LogRequestOver the request is over (either completed or failed)
	LogRequestOver
)

// LogPrinter is used to output the log for a request
type LogPrinter func(trigger LogRequestTrigger, reqRec *logRequestRecord)

type logRequestRecord struct {
	recordIndex    uint64
	startTime      time.Time
	httpRequest    *http.Request
	responseWriter http.ResponseWriter
	funcInfo       *logFuncInfo
	funcInfoMu     sync.RWMutex
	panicError     interface{}
}

type logContextHandler struct {
	logLevel           log.Level
	printLog           LogPrinter
	requestRecordMap   map[uint64]*logRequestRecord
	requestRecordMapMu sync.Mutex
	reqRecordCount     uint64
}

type logFuncInfo struct {
	funcFile      string
	funcFileShort string
	funcLine      int
	funcName      string
	funcNameShort string
}

var funcInfoMap = map[uintptr]*logFuncInfo{}
var funcInfoMapMu sync.RWMutex

func runtimeFuncToInfo(f *runtime.Func) *logFuncInfo {
	file, line := f.FileLine(f.Entry())
	fi := &logFuncInfo{
		funcFile: strings.ReplaceAll(file, "\\", "/"),
		funcLine: line,
		funcName: f.Name(),
	}

	// only keep last 2 names in path
	p1 := strings.LastIndexByte(fi.funcFile, '/')
	if p1 >= 0 {
		p2 := strings.LastIndexByte(fi.funcFile[:p1], '/')
		if p2 >= 0 {
			fi.funcFileShort = fi.funcFile[p2+1:]
		}
	}
	if fi.funcFileShort == "" {
		fi.funcFileShort = fi.funcName
	}

	// remove package prefix. eg: "xxx.com/pkg1/pkg2.foo" => "pkg2.foo"
	p1 = strings.LastIndexByte(fi.funcName, '/')
	if p1 >= 0 {
		fi.funcNameShort = fi.funcName[p1+1:]
	} else {
		fi.funcNameShort = fi.funcName
	}

	// remove ".func?" suffix for anonymous func
	// usually we do not have more than 10 anonymous functions in one function, so this check is enough and fast.
	if len(fi.funcNameShort) > 6 {
		if fi.funcNameShort[len(fi.funcNameShort)-6:len(fi.funcNameShort)-1] == ".func" {
			fi.funcNameShort = fi.funcNameShort[:len(fi.funcNameShort)-6]
		}
	}
	return fi
}

var contextKeyLogRequestRecord interface{} = "logRequestRecord"

//UpdateContextHandlerFuncInfo updates a context's func info by a real handler func `v`
func UpdateContextHandlerFuncInfo(ctx context.Context, v interface{}, friendlyName ...string) {
	if reqRec, ok := ctx.Value(contextKeyLogRequestRecord).(*logRequestRecord); ok {
		var fi *logFuncInfo
		ptr := reflect.ValueOf(v).Pointer()

		funcInfoMapMu.RLock()
		fi, ok = funcInfoMap[ptr]
		funcInfoMapMu.RUnlock()

		if !ok {
			f := runtime.FuncForPC(ptr)
			if f != nil {
				fi = runtimeFuncToInfo(f)
				if len(friendlyName) == 1 {
					fi.funcNameShort = friendlyName[0]
				}

				funcInfoMapMu.Lock()
				funcInfoMap[ptr] = fi
				funcInfoMapMu.Unlock()
			}
		}

		reqRec.funcInfoMu.Lock()
		reqRec.funcInfo = fi
		reqRec.funcInfoMu.Unlock()
	}
}

// WrapContextHandler wraps a log context handler for a router handler
func WrapContextHandler(pathSuffix string, handler http.HandlerFunc, friendlyName ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if !strings.HasPrefix(req.URL.Path, pathSuffix) {
				next.ServeHTTP(resp, req)
				return
			}
			UpdateContextHandlerFuncInfo(req.Context(), handler, friendlyName...)
			handler(resp, req)
		})
	}
}

//UpdateContextHandlerPanicError updates a context's error info, a panic may be recovered by other middlewares, but we still need to know that.
func UpdateContextHandlerPanicError(ctx context.Context, err interface{}) {
	if reqRec, ok := ctx.Value(contextKeyLogRequestRecord).(*logRequestRecord); ok {
		reqRec.panicError = err
	}
}

func (lh *logContextHandler) startSlowQueryDetector(threshold time.Duration) {
	go func(baseCtx context.Context) {
		// This go-routine checks all active requests every second.
		// If a request has been running for a long time (eg: /user/events), we also print a log with "still-executing" message
		// After the "still-executing" log is printed, the record will be removed from the map to prevent from duplicated logs in future
		t := time.NewTicker(time.Second)
		for {
			select {
			case <-baseCtx.Done():
				return
			case <-t.C:
				now := time.Now()
				var slows []*logRequestRecord
				// find all slow requests with lock
				lh.requestRecordMapMu.Lock()
				for i, r := range lh.requestRecordMap {
					d := now.Sub(r.startTime)
					if d >= threshold {
						slows = append(slows, r)
						delete(lh.requestRecordMap, i)
					}
				}
				lh.requestRecordMapMu.Unlock()

				// print logs for slow requests
				if len(slows) > 0 {
					for _, reqRec := range slows {
						lh.printLog(LogRequestExecuting, reqRec)
					}
				}
			}
		}
	}(graceful.GetManager().ShutdownContext())
}

func (lh *logContextHandler) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		reqRec := &logRequestRecord{}
		reqRec.startTime = time.Now()
		reqRec.httpRequest = req
		reqRec.responseWriter = w

		lh.requestRecordMapMu.Lock()
		reqRec.recordIndex = lh.reqRecordCount
		lh.reqRecordCount++
		lh.requestRecordMap[reqRec.recordIndex] = reqRec
		lh.requestRecordMapMu.Unlock()

		defer func() {
			// just in case there is a panic. now the panics are all recovered in middleware.go
			localPanicErr := recover()
			if localPanicErr != nil {
				reqRec.panicError = localPanicErr
			}

			lh.requestRecordMapMu.Lock()
			delete(lh.requestRecordMap, reqRec.recordIndex)
			lh.requestRecordMapMu.Unlock()

			lh.printLog(LogRequestOver, reqRec)

			if localPanicErr != nil {
				// we do not recover any panic, so let the framework handle the panic error
				panic(localPanicErr)
			}
		}()
		req = req.WithContext(context.WithValue(req.Context(), contextKeyLogRequestRecord, reqRec))
		lh.printLog(LogRequestStart, reqRec)
		next.ServeHTTP(w, req)
	})
}

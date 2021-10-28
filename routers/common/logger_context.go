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

// shortenFilename generates a short source code filename from a full package path, eg: "code.gitea.io/routers/common/logger_context.go" => "common/logger_context.go"
func shortenFilename(filename, fallback string) string {
	if filename == "" {
		return fallback
	}
	if lastIndex := strings.LastIndexByte(filename, '/'); lastIndex >= 0 {
		if secondLastIndex := strings.LastIndexByte(filename[:lastIndex], '/'); secondLastIndex >= 0 {
			return filename[secondLastIndex+1:]
		}
	}
	return filename
}

// trimAnonymousFunctionSuffix trims ".func[0-9]*" from the end of anonymous function names, we only want to see the main function names in logs
func trimAnonymousFunctionSuffix(name string) string {
	// if the name is an anonymous name, it should be like "{main-function}.func1", so the length can not be less than 7
	if len(name) < 7 {
		return name
	}

	funcSuffixIndex := strings.LastIndex(name, ".func")
	if funcSuffixIndex < 0 {
		return name
	}

	hasFuncSuffix := true

	// len(".func") = 5
	for i := funcSuffixIndex + 5; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			hasFuncSuffix = false
			break
		}
	}

	if hasFuncSuffix {
		return name[:funcSuffixIndex]
	}
	return name
}

// convertToLogFuncInfo take a runtime.Func and convert it to a logFuncInfo, fill in shorten filename, etc
func convertToLogFuncInfo(f *runtime.Func) *logFuncInfo {
	file, line := f.FileLine(f.Entry())

	info := &logFuncInfo{
		funcFile: strings.ReplaceAll(file, "\\", "/"),
		funcLine: line,
		funcName: f.Name(),
	}

	// only keep last 2 names in path, fall back to funcName if not
	info.funcFileShort = shortenFilename(info.funcFile, info.funcName)

	// remove package prefix. eg: "xxx.com/pkg1/pkg2.foo" => "pkg2.foo"
	pos := strings.LastIndexByte(info.funcName, '/')
	if pos >= 0 {
		info.funcNameShort = info.funcName[pos+1:]
	} else {
		info.funcNameShort = info.funcName
	}

	// remove ".func[0-9]*" suffix for anonymous func
	info.funcNameShort = trimAnonymousFunctionSuffix(info.funcNameShort)

	return info
}

var contextKeyLogRequestRecord interface{} = "logRequestRecord"

//UpdateContextHandlerFuncInfo updates a context's func info by a real handler func `fn`
func UpdateContextHandlerFuncInfo(ctx context.Context, fn interface{}, friendlyName ...string) {
	record, ok := ctx.Value(contextKeyLogRequestRecord).(*logRequestRecord)
	if !ok {
		return
	}

	var info *logFuncInfo

	// ptr represents the memory position of the function passed in as v.
	// This will be used as program counter in FuncForPC below
	ptr := reflect.ValueOf(fn).Pointer()

	// Attempt get pre-cached information for this function pointer
	funcInfoMapMu.RLock()
	info, ok = funcInfoMap[ptr]
	funcInfoMapMu.RUnlock()

	if !ok {
		// This is likely the first time we have seen this function
		//
		// Get the runtime.func for this function (if we can)
		f := runtime.FuncForPC(ptr)
		if f != nil {
			info = convertToLogFuncInfo(f)

			// if we have been provided with a friendlyName override the short name we've generated
			if len(friendlyName) == 1 {
				info.funcNameShort = friendlyName[0]
			}

			// cache this info globally
			funcInfoMapMu.Lock()
			funcInfoMap[ptr] = info
			funcInfoMapMu.Unlock()
		}
	}

	// update our current record
	record.funcInfoMu.Lock()
	record.funcInfo = info
	record.funcInfoMu.Unlock()
}

// WrapContextHandler wraps a log context handler for a router handler
func WrapContextHandler(pathPrefix string, handler http.HandlerFunc, friendlyName ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if !strings.HasPrefix(req.URL.Path, pathPrefix) {
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
	go graceful.GetManager().RunWithShutdownContext(func(baseCtx context.Context) {
		// This go-routine checks all active requests every second.
		// If a request has been running for a long time (eg: /user/events), we also print a log with "still-executing" message
		// After the "still-executing" log is printed, the record will be removed from the map to prevent from duplicated logs in future

		// We do not care about accurate duration here. It just does the check periodically, 0.5s or 1.5ms are all OK.
		t := time.NewTicker(time.Second)
		for {
			select {
			case <-baseCtx.Done():
				return
			case <-t.C:
				now := time.Now()

				var slowRequests []*logRequestRecord

				// find all slow requests with lock
				lh.requestRecordMapMu.Lock()
				for index, reqRecord := range lh.requestRecordMap {
					if now.Sub(reqRecord.startTime) < threshold {
						continue
					}

					slowRequests = append(slowRequests, reqRecord)
					delete(lh.requestRecordMap, index)
				}
				lh.requestRecordMapMu.Unlock()

				// print logs for slow requests
				for _, reqRecord := range slowRequests {
					lh.printLog(LogRequestExecuting, reqRecord)
				}
			}
		}
	})
}

func (lh *logContextHandler) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		record := &logRequestRecord{
			startTime:      time.Now(),
			httpRequest:    req,
			responseWriter: w,
		}

		// generate a record index an insert into the map
		lh.requestRecordMapMu.Lock()
		record.recordIndex = lh.reqRecordCount
		lh.reqRecordCount++
		lh.requestRecordMap[record.recordIndex] = record
		lh.requestRecordMapMu.Unlock()

		defer func() {
			// just in case there is a panic. now the panics are all recovered in middleware.go
			localPanicErr := recover()
			if localPanicErr != nil {
				record.panicError = localPanicErr
			}

			// remove from the record map
			lh.requestRecordMapMu.Lock()
			delete(lh.requestRecordMap, record.recordIndex)
			lh.requestRecordMapMu.Unlock()

			// log the end of the request
			lh.printLog(LogRequestOver, record)

			if localPanicErr != nil {
				// the panic wasn't recovered before us, so we should pass it up, and let the framework handle the panic error
				panic(localPanicErr)
			}
		}()

		req = req.WithContext(context.WithValue(req.Context(), contextKeyLogRequestRecord, record))
		lh.printLog(LogRequestStart, record)
		next.ServeHTTP(w, req)
	})
}

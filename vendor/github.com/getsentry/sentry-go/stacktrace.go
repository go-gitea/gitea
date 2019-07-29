package sentry

import (
	"go/build"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

const unknown string = "unknown"

// The module download is split into two parts: downloading the go.mod and downloading the actual code.
// If you have dependencies only needed for tests, then they will show up in your go.mod,
// and go get will download their go.mods, but it will not download their code.
// The test-only dependencies get downloaded only when you need it, such as the first time you run go test.
//
// https://github.com/golang/go/issues/26913#issuecomment-411976222

// Stacktrace holds information about the frames of the stack.
type Stacktrace struct {
	Frames        []Frame `json:"frames,omitempty"`
	FramesOmitted []uint  `json:"frames_omitted,omitempty"`
}

// NewStacktrace creates a stacktrace using `runtime.Callers`.
func NewStacktrace() *Stacktrace {
	pcs := make([]uintptr, 100)
	n := runtime.Callers(1, pcs)

	if n == 0 {
		return nil
	}

	frames := extractFrames(pcs[:n])
	frames = filterFrames(frames)

	stacktrace := Stacktrace{
		Frames: frames,
	}

	return &stacktrace
}

// ExtractStacktrace creates a new `Stacktrace` based on the given `error` object.
// TODO: Make it configurable so that anyone can provide their own implementation?
// Use of reflection allows us to not have a hard dependency on any given package, so we don't have to import it
func ExtractStacktrace(err error) *Stacktrace {
	method := extractReflectedStacktraceMethod(err)

	if !method.IsValid() {
		return nil
	}

	pcs := extractPcs(method)

	if len(pcs) == 0 {
		return nil
	}

	frames := extractFrames(pcs)
	frames = filterFrames(frames)

	stacktrace := Stacktrace{
		Frames: frames,
	}

	return &stacktrace
}

func extractReflectedStacktraceMethod(err error) reflect.Value {
	var method reflect.Value

	// https://github.com/pingcap/errors
	methodGetStackTracer := reflect.ValueOf(err).MethodByName("GetStackTracer")
	// https://github.com/pkg/errors
	methodStackTrace := reflect.ValueOf(err).MethodByName("StackTrace")
	// https://github.com/go-errors/errors
	methodStackFrames := reflect.ValueOf(err).MethodByName("StackFrames")

	if methodGetStackTracer.IsValid() {
		stacktracer := methodGetStackTracer.Call(make([]reflect.Value, 0))[0]
		stacktracerStackTrace := reflect.ValueOf(stacktracer).MethodByName("StackTrace")

		if stacktracerStackTrace.IsValid() {
			method = stacktracerStackTrace
		}
	}

	if methodStackTrace.IsValid() {
		method = methodStackTrace
	}

	if methodStackFrames.IsValid() {
		method = methodStackFrames
	}

	return method
}

func extractPcs(method reflect.Value) []uintptr {
	var pcs []uintptr

	stacktrace := method.Call(make([]reflect.Value, 0))[0]

	if stacktrace.Kind() != reflect.Slice {
		return nil
	}

	for i := 0; i < stacktrace.Len(); i++ {
		pc := stacktrace.Index(i)

		if pc.Kind() == reflect.Uintptr {
			pcs = append(pcs, uintptr(pc.Uint()))
			continue
		}

		if pc.Kind() == reflect.Struct {
			field := pc.FieldByName("ProgramCounter")
			if field.IsValid() && field.Kind() == reflect.Uintptr {
				pcs = append(pcs, uintptr(field.Uint()))
				continue
			}
		}
	}

	return pcs
}

// https://docs.sentry.io/development/sdk-dev/interfaces/stacktrace/
type Frame struct {
	Function    string                 `json:"function,omitempty"`
	Symbol      string                 `json:"symbol,omitempty"`
	Module      string                 `json:"module,omitempty"`
	Package     string                 `json:"package,omitempty"`
	Filename    string                 `json:"filename,omitempty"`
	AbsPath     string                 `json:"abs_path,omitempty"`
	Lineno      int                    `json:"lineno,omitempty"`
	Colno       int                    `json:"colno,omitempty"`
	PreContext  []string               `json:"pre_context,omitempty"`
	ContextLine string                 `json:"context_line,omitempty"`
	PostContext []string               `json:"post_context,omitempty"`
	InApp       bool                   `json:"in_app,omitempty"`
	Vars        map[string]interface{} `json:"vars,omitempty"`
}

// NewFrame assembles a stacktrace frame out of `runtime.Frame`.
func NewFrame(f runtime.Frame) Frame {
	abspath := f.File
	filename := f.File
	function := f.Function
	var module string

	if filename != "" {
		filename = extractFilename(filename)
	} else {
		filename = unknown
	}

	if abspath == "" {
		abspath = unknown
	}

	if function != "" {
		module, function = deconstructFunctionName(function)
	}

	frame := Frame{
		AbsPath:  abspath,
		Filename: filename,
		Lineno:   f.Line,
		Module:   module,
		Function: function,
	}

	frame.InApp = isInAppFrame(frame)

	return frame
}

func extractFrames(pcs []uintptr) []Frame {
	var frames []Frame
	callersFrames := runtime.CallersFrames(pcs)

	for {
		callerFrame, more := callersFrames.Next()

		frames = append([]Frame{
			NewFrame(callerFrame),
		}, frames...)

		if !more {
			break
		}
	}

	return frames
}

func filterFrames(frames []Frame) []Frame {
	isTestFileRegexp := regexp.MustCompile(`getsentry/sentry-go/.+_test.go`)
	isExampleFileRegexp := regexp.MustCompile(`getsentry/sentry-go/example/`)
	filteredFrames := make([]Frame, 0, len(frames))

	for _, frame := range frames {
		// go runtime frames
		if frame.Module == "runtime" || frame.Module == "testing" {
			continue
		}
		// sentry internal frames
		isTestFile := isTestFileRegexp.MatchString(frame.AbsPath)
		isExampleFile := isExampleFileRegexp.MatchString(frame.AbsPath)
		if strings.Contains(frame.AbsPath, "github.com/getsentry/sentry-go") &&
			!isTestFile &&
			!isExampleFile {
			continue
		}
		filteredFrames = append(filteredFrames, frame)
	}

	return filteredFrames
}

func extractFilename(path string) string {
	_, file := filepath.Split(path)
	return file
}

func isInAppFrame(frame Frame) bool {
	if strings.HasPrefix(frame.AbsPath, build.Default.GOROOT) ||
		strings.Contains(frame.Module, "vendor") ||
		strings.Contains(frame.Module, "third_party") {
		return false
	}

	return true
}

// Transform `runtime/debug.*T·ptrmethod` into `{ module: runtime/debug, function: *T.ptrmethod }`
func deconstructFunctionName(name string) (module string, function string) {
	if idx := strings.LastIndex(name, "."); idx != -1 {
		module = name[:idx]
		function = name[idx+1:]
	}
	function = strings.Replace(function, "·", ".", -1)
	return module, function
}

func callerFunctionName() string {
	pcs := make([]uintptr, 1)
	runtime.Callers(3, pcs)
	callersFrames := runtime.CallersFrames(pcs)
	callerFrame, _ := callersFrames.Next()
	_, function := deconstructFunctionName(callerFrame.Function)
	return function
}

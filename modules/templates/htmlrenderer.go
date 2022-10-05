// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/watcher"

	"github.com/unrolled/render"
)

var (
	rendererKey interface{} = "templatesHtmlRenderer"

	templateError    = regexp.MustCompile(`^template: (.*):([0-9]+): (.*)`)
	notDefinedError  = regexp.MustCompile(`^template: (.*):([0-9]+): function "(.*)" not defined`)
	unexpectedError  = regexp.MustCompile(`^template: (.*):([0-9]+): unexpected "(.*)" in operand`)
	expectedEndError = regexp.MustCompile(`^template: (.*):([0-9]+): expected end; found (.*)`)
)

// HTMLRenderer returns the current html renderer for the context or creates and stores one within the context for future use
func HTMLRenderer(ctx context.Context) (context.Context, *render.Render) {
	rendererInterface := ctx.Value(rendererKey)
	if rendererInterface != nil {
		renderer, ok := rendererInterface.(*render.Render)
		if ok {
			return ctx, renderer
		}
	}

	rendererType := "static"
	if !setting.IsProd {
		rendererType = "auto-reloading"
	}
	log.Log(1, log.DEBUG, "Creating "+rendererType+" HTML Renderer")

	compilingTemplates := true
	defer func() {
		if !compilingTemplates {
			return
		}

		panicked := recover()
		if panicked == nil {
			return
		}

		// OK try to handle the panic...
		err, ok := panicked.(error)
		if ok {
			handlePanicError(err)
		}
		log.Fatal("PANIC: Unable to compile templates: %v\nStacktrace:\n%s", panicked, log.Stack(2))
	}()

	renderer := render.New(render.Options{
		Extensions:                []string{".tmpl"},
		Directory:                 "templates",
		Funcs:                     NewFuncMap(),
		Asset:                     GetAsset,
		AssetNames:                GetTemplateAssetNames,
		UseMutexLock:              !setting.IsProd,
		IsDevelopment:             false,
		DisableHTTPErrorRendering: true,
	})
	compilingTemplates = false
	if !setting.IsProd {
		watcher.CreateWatcher(ctx, "HTML Templates", &watcher.CreateWatcherOpts{
			PathsCallback:   walkTemplateFiles,
			BetweenCallback: renderer.CompileTemplates,
		})
	}
	return context.WithValue(ctx, rendererKey, renderer), renderer
}

func handlePanicError(err error) {
	wrapFatal(handleNotDefinedPanicError(err))
	wrapFatal(handleUnexpected(err))
	wrapFatal(handleExpectedEnd(err))
	wrapFatal(handleGenericTemplateError(err))
}

func wrapFatal(format string, args ...interface{}) {
	if format == "" {
		return
	}
	log.Fatal(format, args...)
}

func handleGenericTemplateError(err error) (string, []interface{}) {
	groups := templateError.FindStringSubmatch(err.Error())
	if len(groups) != 4 {
		return "", nil
	}

	templateName, lineNumberStr, message := groups[1], groups[2], groups[3]

	filename, assetErr := GetAssetFilename("templates/" + templateName + ".tmpl")
	if assetErr != nil {
		return "", nil
	}

	lineNumber, _ := strconv.Atoi(lineNumberStr)

	line := getLineFromAsset(templateName, lineNumber, "")

	return "PANIC: Unable to compile templates due to: %s in template file %s at line %d:\n%s\nStacktrace:\n%s", []interface{}{message, filename, lineNumber, log.NewColoredValue(line, log.Reset), log.Stack(2)}
}

func handleNotDefinedPanicError(err error) (string, []interface{}) {
	groups := notDefinedError.FindStringSubmatch(err.Error())
	if len(groups) != 4 {
		return "", nil
	}

	templateName, lineNumberStr, functionName := groups[1], groups[2], groups[3]

	functionName, _ = strconv.Unquote(`"` + functionName + `"`)

	filename, assetErr := GetAssetFilename("templates/" + templateName + ".tmpl")
	if assetErr != nil {
		return "", nil
	}

	lineNumber, _ := strconv.Atoi(lineNumberStr)

	line := getLineFromAsset(templateName, lineNumber, functionName)

	return "PANIC: Unable to compile templates due to undefined function %q in template file %s at line %d:\n%s\nStacktrace:\n%s", []interface{}{functionName, filename, lineNumber, log.NewColoredValue(line, log.Reset), log.Stack(2)}
}

func handleUnexpected(err error) (string, []interface{}) {
	groups := unexpectedError.FindStringSubmatch(err.Error())
	if len(groups) != 4 {
		return "", nil
	}

	templateName, lineNumberStr, unexpected := groups[1], groups[2], groups[3]
	unexpected, _ = strconv.Unquote(`"` + unexpected + `"`)

	filename, assetErr := GetAssetFilename("templates/" + templateName + ".tmpl")
	if assetErr != nil {
		return "", nil
	}

	lineNumber, _ := strconv.Atoi(lineNumberStr)

	line := getLineFromAsset(templateName, lineNumber, unexpected)

	return "PANIC: Unable to compile templates due to unexpected %q in template file %s at line %d:\n%s\nStacktrace:\n%s", []interface{}{unexpected, filename, lineNumber, log.NewColoredValue(line, log.Reset), log.Stack(2)}
}

func handleExpectedEnd(err error) (string, []interface{}) {
	groups := expectedEndError.FindStringSubmatch(err.Error())
	if len(groups) != 4 {
		return "", nil
	}

	templateName, lineNumberStr, unexpected := groups[1], groups[2], groups[3]

	filename, assetErr := GetAssetFilename("templates/" + templateName + ".tmpl")
	if assetErr != nil {
		return "", nil
	}

	lineNumber, _ := strconv.Atoi(lineNumberStr)

	line := getLineFromAsset(templateName, lineNumber, unexpected)

	return "PANIC: Unable to compile templates due to missing end with unexpected %q in template file %s at line %d:\n%s\nStacktrace:\n%s", []interface{}{unexpected, filename, lineNumber, log.NewColoredValue(line, log.Reset), log.Stack(2)}
}

func getLineFromAsset(templateName string, lineNumber int, functionName string) string {
	bs, err := GetAsset("templates/" + templateName + ".tmpl")
	if err != nil {
		return fmt.Sprintf("(unable to read template file: %v)", err)
	}

	sb := &strings.Builder{}
	start := 0
	var lineBs []byte
	for i := 0; i < lineNumber && start < len(bs); i++ {
		end := bytes.IndexByte(bs[start:], '\n')
		if end < 0 {
			end = len(bs)
		} else {
			end += start
		}
		lineBs = bs[start:end]
		if lineNumber-i < 4 {
			_, _ = sb.Write(lineBs)
			_ = sb.WriteByte('\n')
		}
		start = end + 1
	}

	if functionName != "" {
		idx := strings.Index(string(lineBs), functionName)
		for i := range lineBs[:idx] {
			if lineBs[i] != '\t' {
				lineBs[i] = ' '
			}
		}
		_, _ = sb.Write(lineBs[:idx])
		if idx >= 0 {
			_, _ = sb.WriteString(strings.Repeat("^", len(functionName)))
		}
		_ = sb.WriteByte('\n')
	}

	return strings.Repeat("-", 70) + "\n" + sb.String() + strings.Repeat("-", 70)
}

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
		log.Fatal("PANIC: Unable to compile templates!\n%v\n\nStacktrace:\n%s", panicked, log.Stack(2))
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

func wrapFatal(format string, args []interface{}) {
	if format == "" {
		return
	}
	log.FatalWithSkip(1, format, args...)
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

	return "PANIC: Unable to compile templates!\n%s in template file %s at line %d:\n\n%s\nStacktrace:\n\n%s", []interface{}{message, filename, lineNumber, log.NewColoredValue(line, log.Reset), log.Stack(2)}
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

	return "PANIC: Unable to compile templates!\nUndefined function %q in template file %s at line %d:\n\n%s", []interface{}{functionName, filename, lineNumber, log.NewColoredValue(line, log.Reset)}
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

	return "PANIC: Unable to compile templates!\nUnexpected %q in template file %s at line %d:\n\n%s", []interface{}{unexpected, filename, lineNumber, log.NewColoredValue(line, log.Reset)}
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

	return "PANIC: Unable to compile templates!\nMissing end with unexpected %q in template file %s at line %d:\n\n%s", []interface{}{unexpected, filename, lineNumber, log.NewColoredValue(line, log.Reset)}
}

const dashSeparator = "----------------------------------------------------------------------\n"

func getLineFromAsset(templateName string, targetLineNum int, target string) string {
	bs, err := GetAsset("templates/" + templateName + ".tmpl")
	if err != nil {
		return fmt.Sprintf("(unable to read template file: %v)", err)
	}

	sb := &strings.Builder{}

	// Write the header
	sb.WriteString(dashSeparator)

	var lineBs []byte

	// Iterate through the lines from the asset file to find the target line
	for start, currentLineNum := 0, 1; currentLineNum <= targetLineNum && start < len(bs); currentLineNum++ {
		// Find the next new line
		end := bytes.IndexByte(bs[start:], '\n')

		// adjust the end to be a direct pointer in to []byte
		if end < 0 {
			end = len(bs)
		} else {
			end += start
		}

		// set lineBs to the current line []byte
		lineBs = bs[start:end]

		// move start to after the current new line position
		start = end + 1

		// Write 2 preceding lines + the target line
		if targetLineNum-currentLineNum < 3 {
			_, _ = sb.Write(lineBs)
			_ = sb.WriteByte('\n')
		}
	}

	// If there is a provided target to look for in the line add a pointer to it
	// e.g.                                                        ^^^^^^^
	if target != "" {
		idx := bytes.Index(lineBs, []byte(target))

		if idx >= 0 {
			// take the current line and replace preceding text with whitespace (except for tab)
			for i := range lineBs[:idx] {
				if lineBs[i] != '\t' {
					lineBs[i] = ' '
				}
			}

			// write the preceding "space"
			_, _ = sb.Write(lineBs[:idx])

			// Now write the ^^ pointer
			_, _ = sb.WriteString(strings.Repeat("^", len(target)))
			_ = sb.WriteByte('\n')
		}
	}

	// Finally write the footer
	sb.WriteString(dashSeparator)

	return sb.String()
}

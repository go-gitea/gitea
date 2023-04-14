// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	texttemplate "text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var (
	rendererKey interface{} = "templatesHtmlRenderer"

	templateError    = regexp.MustCompile(`^template: (.*):([0-9]+): (.*)`)
	notDefinedError  = regexp.MustCompile(`^template: (.*):([0-9]+): function "(.*)" not defined`)
	unexpectedError  = regexp.MustCompile(`^template: (.*):([0-9]+): unexpected "(.*)" in operand`)
	expectedEndError = regexp.MustCompile(`^template: (.*):([0-9]+): expected end; found (.*)`)
)

type HTMLRender struct {
	templates atomic.Pointer[template.Template]
}

var ErrTemplateNotInitialized = errors.New("template system is not initialized, check your log for errors")

func (h *HTMLRender) HTML(w io.Writer, status int, name string, data interface{}) error {
	if respWriter, ok := w.(http.ResponseWriter); ok {
		if respWriter.Header().Get("Content-Type") == "" {
			respWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		respWriter.WriteHeader(status)
	}
	t, err := h.TemplateLookup(name)
	if err != nil {
		return texttemplate.ExecError{Name: name, Err: err}
	}
	return t.Execute(w, data)
}

func (h *HTMLRender) TemplateLookup(name string) (*template.Template, error) {
	tmpls := h.templates.Load()
	if tmpls == nil {
		return nil, ErrTemplateNotInitialized
	}
	tmpl := tmpls.Lookup(name)
	if tmpl == nil {
		return nil, util.ErrNotExist
	}
	return tmpl, nil
}

func (h *HTMLRender) CompileTemplates() error {
	extSuffix := ".tmpl"
	tmpls := template.New("")
	assets := AssetFS()
	files, err := ListWebTemplateAssetNames(assets)
	if err != nil {
		return nil
	}
	for _, file := range files {
		if !strings.HasSuffix(file, extSuffix) {
			continue
		}
		name := strings.TrimSuffix(file, extSuffix)
		tmpl := tmpls.New(filepath.ToSlash(name))
		for _, fm := range NewFuncMap() {
			tmpl.Funcs(fm)
		}
		buf, err := assets.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err = tmpl.Parse(string(buf)); err != nil {
			return err
		}
	}
	h.templates.Store(tmpls)
	return nil
}

// HTMLRenderer returns the current html renderer for the context or creates and stores one within the context for future use
func HTMLRenderer(ctx context.Context) (context.Context, *HTMLRender) {
	if renderer, ok := ctx.Value(rendererKey).(*HTMLRender); ok {
		return ctx, renderer
	}

	rendererType := "static"
	if !setting.IsProd {
		rendererType = "auto-reloading"
	}
	log.Log(1, log.DEBUG, "Creating "+rendererType+" HTML Renderer")

	renderer := &HTMLRender{}
	if err := renderer.CompileTemplates(); err != nil {
		wrapFatal(handleNotDefinedPanicError(err))
		wrapFatal(handleUnexpected(err))
		wrapFatal(handleExpectedEnd(err))
		wrapFatal(handleGenericTemplateError(err))
		log.Fatal("HTMLRenderer error: %v", err)
	}
	if !setting.IsProd {
		go AssetFS().WatchLocalChanges(ctx, func() {
			if err := renderer.CompileTemplates(); err != nil {
				log.Error("Template error: %v\n%s", err, log.Stack(2))
			}
		})
	}
	return context.WithValue(ctx, rendererKey, renderer), renderer
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
	filename := fmt.Sprintf("%s (provided by %s)", templateName, AssetFS().GetFileLayerName(templateName+".tmpl"))
	lineNumber, _ := strconv.Atoi(lineNumberStr)
	line := GetLineFromTemplate(templateName, lineNumber, "", -1)

	return "PANIC: Unable to compile templates!\n%s in template file %s at line %d:\n\n%s\nStacktrace:\n\n%s", []interface{}{message, filename, lineNumber, log.NewColoredValue(line, log.Reset), log.Stack(2)}
}

func handleNotDefinedPanicError(err error) (string, []interface{}) {
	groups := notDefinedError.FindStringSubmatch(err.Error())
	if len(groups) != 4 {
		return "", nil
	}

	templateName, lineNumberStr, functionName := groups[1], groups[2], groups[3]
	functionName, _ = strconv.Unquote(`"` + functionName + `"`)
	filename := fmt.Sprintf("%s (provided by %s)", templateName, AssetFS().GetFileLayerName(templateName+".tmpl"))
	lineNumber, _ := strconv.Atoi(lineNumberStr)
	line := GetLineFromTemplate(templateName, lineNumber, functionName, -1)

	return "PANIC: Unable to compile templates!\nUndefined function %q in template file %s at line %d:\n\n%s", []interface{}{functionName, filename, lineNumber, log.NewColoredValue(line, log.Reset)}
}

func handleUnexpected(err error) (string, []interface{}) {
	groups := unexpectedError.FindStringSubmatch(err.Error())
	if len(groups) != 4 {
		return "", nil
	}

	templateName, lineNumberStr, unexpected := groups[1], groups[2], groups[3]
	unexpected, _ = strconv.Unquote(`"` + unexpected + `"`)
	filename := fmt.Sprintf("%s (provided by %s)", templateName, AssetFS().GetFileLayerName(templateName+".tmpl"))
	lineNumber, _ := strconv.Atoi(lineNumberStr)
	line := GetLineFromTemplate(templateName, lineNumber, unexpected, -1)

	return "PANIC: Unable to compile templates!\nUnexpected %q in template file %s at line %d:\n\n%s", []interface{}{unexpected, filename, lineNumber, log.NewColoredValue(line, log.Reset)}
}

func handleExpectedEnd(err error) (string, []interface{}) {
	groups := expectedEndError.FindStringSubmatch(err.Error())
	if len(groups) != 4 {
		return "", nil
	}

	templateName, lineNumberStr, unexpected := groups[1], groups[2], groups[3]
	filename := fmt.Sprintf("%s (provided by %s)", templateName, AssetFS().GetFileLayerName(templateName+".tmpl"))
	lineNumber, _ := strconv.Atoi(lineNumberStr)
	line := GetLineFromTemplate(templateName, lineNumber, unexpected, -1)

	return "PANIC: Unable to compile templates!\nMissing end with unexpected %q in template file %s at line %d:\n\n%s", []interface{}{unexpected, filename, lineNumber, log.NewColoredValue(line, log.Reset)}
}

const dashSeparator = "----------------------------------------------------------------------\n"

// GetLineFromTemplate returns a line from a template with some context
func GetLineFromTemplate(templateName string, targetLineNum int, target string, position int) string {
	bs, err := AssetFS().ReadFile(templateName + ".tmpl")
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

	// FIXME: this algorithm could provide incorrect results and mislead the developers.
	// For example: Undefined function "file" in template .....
	//     {{Func .file.Addition file.Deletion .file.Addition}}
	//             ^^^^          ^(the real error is here)
	// The pointer is added to the first one, but the second one is the real incorrect one.
	//
	// If there is a provided target to look for in the line add a pointer to it
	// e.g.                                                        ^^^^^^^
	if target != "" {
		targetPos := bytes.Index(lineBs, []byte(target))
		if targetPos >= 0 {
			position = targetPos
		}
	}
	if position >= 0 {
		// take the current line and replace preceding text with whitespace (except for tab)
		for i := range lineBs[:position] {
			if lineBs[i] != '\t' {
				lineBs[i] = ' '
			}
		}

		// write the preceding "space"
		_, _ = sb.Write(lineBs[:position])

		// Now write the ^^ pointer
		targetLen := len(target)
		if targetLen == 0 {
			targetLen = 1
		}
		_, _ = sb.WriteString(strings.Repeat("^", targetLen))
		_ = sb.WriteByte('\n')
	}

	// Finally write the footer
	sb.WriteString(dashSeparator)

	return sb.String()
}

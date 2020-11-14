// Copyright @2017 Saddam Hossain.  All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package renderer implements frequently usages response render methods like JSON, JSONP, XML, YAML, HTML, FILE etc
// This package is useful when building REST api and to provide response to consumer

// Package renderer documentaton contains the API for this package please follow the link below
package renderer

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

const (
	// ContentType represents content type
	ContentType string = "Content-Type"
	// ContentJSON represents content type application/json
	ContentJSON string = "application/json"
	// ContentJSONP represents content type application/javascript
	ContentJSONP string = "application/javascript"
	// ContentXML represents content type application/xml
	ContentXML string = "application/xml"
	// ContentYAML represents content type application/x-yaml
	ContentYAML string = "application/x-yaml"
	// ContentHTML represents content type text/html
	ContentHTML string = "text/html"
	// ContentText represents content type text/plain
	ContentText string = "text/plain"
	// ContentBinary represents content type application/octet-stream
	ContentBinary string = "application/octet-stream"

	// ContentDisposition describes contentDisposition
	ContentDisposition string = "Content-Disposition"
	// contentDispositionInline describes content disposition type
	contentDispositionInline string = "inline"
	// contentDispositionAttachment describes content disposition type
	contentDispositionAttachment string = "attachment"

	defaultCharSet            string = "utf-8"
	defaultJSONPrefix         string = ""
	defaultXMLPrefix          string = `<?xml version="1.0" encoding="ISO-8859-1" ?>\n`
	defaultTemplateExt        string = "tpl"
	defaultLayoutExt          string = "lout"
	defaultTemplateLeftDelim  string = "{{"
	defaultTemplateRightDelim string = "}}"
)

type (
	// M describes handy type that represents data to send as response
	M map[string]interface{}

	// Options describes an option type
	Options struct {
		// Charset represents the Response charset; default: utf-8
		Charset string
		// ContentJSON represents the Content-Type for JSON
		ContentJSON string
		// ContentJSONP represents the Content-Type for JSONP
		ContentJSONP string
		// ContentXML represents the Content-Type for XML
		ContentXML string
		// ContentYAML represents the Content-Type for YAML
		ContentYAML string
		// ContentHTML represents the Content-Type for HTML
		ContentHTML string
		// ContentText represents the Content-Type for Text
		ContentText string
		// ContentBinary represents the Content-Type for octet-stream
		ContentBinary string

		// UnEscapeHTML set UnEscapeHTML for JSON; default false
		UnEscapeHTML bool
		// DisableCharset set DisableCharset in Response Content-Type
		DisableCharset bool
		// Debug set the debug mode. if debug is true then every time "VIEW" call parse the templates
		Debug bool
		// JSONIndent set JSON Indent in response; default false
		JSONIndent bool
		// XMLIndent set XML Indent in response; default false
		XMLIndent bool

		// JSONPrefix set Prefix in JSON response
		JSONPrefix string
		// XMLPrefix set Prefix in XML response
		XMLPrefix string

		// TemplateDir set the Template directory
		TemplateDir string
		// TemplateExtension set the Template extension
		TemplateExtension string
		// LeftDelim set template left delimiter default is {{
		LeftDelim string
		// RightDelim set template right delimiter default is }}
		RightDelim string
		// LayoutExtension set the Layout extension
		LayoutExtension string
		// FuncMap contain function map for template
		FuncMap []template.FuncMap
		// ParseGlobPattern contain parse glob pattern
		ParseGlobPattern string
	}

	// Render describes a renderer type
	Render struct {
		opts          Options
		templates     map[string]*template.Template
		globTemplates *template.Template
		headers       map[string]string
	}
)

// New return a new instance of a pointer to Render
func New(opts ...Options) *Render {
	var opt Options
	if opts != nil {
		opt = opts[0]
	}

	r := &Render{
		opts:      opt,
		templates: make(map[string]*template.Template),
	}

	// build options for the Render instance
	r.buildOptions()

	// if TemplateDir is not empty then call the parseTemplates
	if r.opts.TemplateDir != "" {
		r.parseTemplates()
	}

	// ParseGlobPattern is not empty then parse template with pattern
	if r.opts.ParseGlobPattern != "" {
		r.parseGlob()
	}

	return r
}

// buildOptions builds the options and set deault values for options
func (r *Render) buildOptions() {
	if r.opts.Charset == "" {
		r.opts.Charset = defaultCharSet
	}

	if r.opts.JSONPrefix == "" {
		r.opts.JSONPrefix = defaultJSONPrefix
	}

	if r.opts.XMLPrefix == "" {
		r.opts.XMLPrefix = defaultXMLPrefix
	}

	if r.opts.TemplateExtension == "" {
		r.opts.TemplateExtension = "." + defaultTemplateExt
	} else {
		r.opts.TemplateExtension = "." + r.opts.TemplateExtension
	}

	if r.opts.LayoutExtension == "" {
		r.opts.LayoutExtension = "." + defaultLayoutExt
	} else {
		r.opts.LayoutExtension = "." + r.opts.LayoutExtension
	}

	if r.opts.LeftDelim == "" {
		r.opts.LeftDelim = defaultTemplateLeftDelim
	}

	if r.opts.RightDelim == "" {
		r.opts.RightDelim = defaultTemplateRightDelim
	}

	r.opts.ContentJSON = ContentJSON
	r.opts.ContentJSONP = ContentJSONP
	r.opts.ContentXML = ContentXML
	r.opts.ContentYAML = ContentYAML
	r.opts.ContentHTML = ContentHTML
	r.opts.ContentText = ContentText
	r.opts.ContentBinary = ContentBinary

	if !r.opts.DisableCharset {
		r.enableCharset()
	}
}

func (r *Render) enableCharset() {
	r.opts.ContentJSON = fmt.Sprintf("%s; charset=%s", r.opts.ContentJSON, r.opts.Charset)
	r.opts.ContentJSONP = fmt.Sprintf("%s; charset=%s", r.opts.ContentJSONP, r.opts.Charset)
	r.opts.ContentXML = fmt.Sprintf("%s; charset=%s", r.opts.ContentXML, r.opts.Charset)
	r.opts.ContentYAML = fmt.Sprintf("%s; charset=%s", r.opts.ContentYAML, r.opts.Charset)
	r.opts.ContentHTML = fmt.Sprintf("%s; charset=%s", r.opts.ContentHTML, r.opts.Charset)
	r.opts.ContentText = fmt.Sprintf("%s; charset=%s", r.opts.ContentText, r.opts.Charset)
	r.opts.ContentBinary = fmt.Sprintf("%s; charset=%s", r.opts.ContentBinary, r.opts.Charset)
}

// DisableCharset change the DisableCharset for JSON on the fly
func (r *Render) DisableCharset(b bool) *Render {
	r.opts.DisableCharset = b
	if !b {
		r.buildOptions()
	}
	return r
}

// JSONIndent change the JSONIndent for JSON on the fly
func (r *Render) JSONIndent(b bool) *Render {
	r.opts.JSONIndent = b
	return r
}

// XMLIndent change the XMLIndent for XML on the fly
func (r *Render) XMLIndent(b bool) *Render {
	r.opts.XMLIndent = b
	return r
}

// Charset change the Charset for response on the fly
func (r *Render) Charset(c string) *Render {
	r.opts.Charset = c
	return r
}

// EscapeHTML change the EscapeHTML for JSON on the fly
func (r *Render) EscapeHTML(b bool) *Render {
	r.opts.UnEscapeHTML = b
	return r
}

// Delims set template delimiter on the fly
func (r *Render) Delims(left, right string) *Render {
	r.opts.LeftDelim = left
	r.opts.RightDelim = right
	return r
}

// FuncMap set template FuncMap on the fly
func (r *Render) FuncMap(fmap template.FuncMap) *Render {
	r.opts.FuncMap = append(r.opts.FuncMap, fmap)
	return r
}

// NoContent serve success but no content response
func (r *Render) NoContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Render serve raw response where you have to build the headers, body
func (r *Render) Render(w http.ResponseWriter, status int, v interface{}) error {
	w.WriteHeader(status)
	_, err := w.Write(v.([]byte))
	return err
}

// String serve string content as text/plain response
func (r *Render) String(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set(ContentType, r.opts.ContentText)
	w.WriteHeader(status)
	_, err := w.Write([]byte(v.(string)))
	return err
}

// json converts the data as bytes using json encoder
func (r *Render) json(v interface{}) ([]byte, error) {
	var bs []byte
	var err error
	if r.opts.JSONIndent {
		bs, err = json.MarshalIndent(v, "", " ")
	} else {
		bs, err = json.Marshal(v)
	}
	if err != nil {
		return bs, err
	}
	if r.opts.UnEscapeHTML {
		bs = bytes.Replace(bs, []byte("\\u003c"), []byte("<"), -1)
		bs = bytes.Replace(bs, []byte("\\u003e"), []byte(">"), -1)
		bs = bytes.Replace(bs, []byte("\\u0026"), []byte("&"), -1)
	}
	return bs, nil
}

// JSON serve data as JSON as response
func (r *Render) JSON(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set(ContentType, r.opts.ContentJSON)
	w.WriteHeader(status)

	bs, err := r.json(v)
	if err != nil {
		return err
	}
	if r.opts.JSONPrefix != "" {
		w.Write([]byte(r.opts.JSONPrefix))
	}
	_, err = w.Write(bs)
	return err
}

// JSONP serve data as JSONP response
func (r *Render) JSONP(w http.ResponseWriter, status int, callback string, v interface{}) error {
	w.Header().Set(ContentType, r.opts.ContentJSONP)
	w.WriteHeader(status)

	bs, err := r.json(v)
	if err != nil {
		return err
	}

	if callback == "" {
		return errors.New("renderer: callback can not bet empty")
	}

	w.Write([]byte(callback + "("))
	_, err = w.Write(bs)
	w.Write([]byte(");"))

	return err
}

// XML serve data as XML response
func (r *Render) XML(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set(ContentType, r.opts.ContentXML)
	w.WriteHeader(status)
	var bs []byte
	var err error

	if r.opts.XMLIndent {
		bs, err = xml.MarshalIndent(v, "", " ")
	} else {
		bs, err = xml.Marshal(v)
	}
	if err != nil {
		return err
	}
	if r.opts.XMLPrefix != "" {
		w.Write([]byte(r.opts.XMLPrefix))
	}
	_, err = w.Write(bs)
	return err
}

// YAML serve data as YAML response
func (r *Render) YAML(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set(ContentType, r.opts.ContentYAML)
	w.WriteHeader(status)

	bs, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(bs)
	return err
}

// HTMLString render string as html. Note: You must provide trusted html when using this method
func (r *Render) HTMLString(w http.ResponseWriter, status int, html string) error {
	w.Header().Set(ContentType, r.opts.ContentHTML)
	w.WriteHeader(status)
	out := template.HTML(html)
	_, err := w.Write([]byte(out))
	return err
}

// HTML render html from template.Glob patterns and execute template by name. See README.md for detail example.
func (r *Render) HTML(w http.ResponseWriter, status int, name string, v interface{}) error {
	w.Header().Set(ContentType, r.opts.ContentHTML)
	w.WriteHeader(status)

	if name == "" {
		return errors.New("renderer: template name not exist")
	}

	if r.opts.Debug {
		r.parseGlob()
	}

	buf := new(bytes.Buffer)
	defer buf.Reset()

	if err := r.globTemplates.ExecuteTemplate(buf, name, v); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// Template build html from template and serve html content as response. See README.md for detail example.
func (r *Render) Template(w http.ResponseWriter, status int, tpls []string, v interface{}) error {
	w.Header().Set(ContentType, r.opts.ContentHTML)
	w.WriteHeader(status)

	tmain := template.New(filepath.Base(tpls[0]))
	tmain.Delims(r.opts.LeftDelim, r.opts.RightDelim)
	for _, fm := range r.opts.FuncMap {
		tmain.Funcs(fm)
	}
	t := template.Must(tmain.ParseFiles(tpls...))

	buf := new(bytes.Buffer)
	defer buf.Reset()

	if err := t.Execute(buf, v); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// View build html from template directory and serve html content as response. See README.md for detail example.
func (r *Render) View(w http.ResponseWriter, status int, name string, v interface{}) error {
	w.Header().Set(ContentType, r.opts.ContentHTML)
	w.WriteHeader(status)

	buf := new(bytes.Buffer)
	defer buf.Reset()

	if r.opts.Debug {
		r.parseTemplates()
	}

	name += r.opts.TemplateExtension
	tmpl, ok := r.templates[name]
	if !ok {
		return fmt.Errorf("renderer: template %s does not exist", name)
	}

	if err := tmpl.Execute(buf, v); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

// Binary serve file as application/octet-stream response; you may add ContentDisposition by your own.
func (r *Render) Binary(w http.ResponseWriter, status int, reader io.Reader, filename string, inline bool) error {
	if inline {
		w.Header().Set(ContentDisposition, fmt.Sprintf("%s; filename=%s", contentDispositionInline, filename))
	} else {
		w.Header().Set(ContentDisposition, fmt.Sprintf("%s; filename=%s", contentDispositionAttachment, filename))
	}
	w.Header().Set(ContentType, r.opts.ContentBinary)
	w.WriteHeader(status)
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	_, err = w.Write(bs)
	return err
}

// File serve file as response from io.Reader
func (r *Render) File(w http.ResponseWriter, status int, reader io.Reader, filename string, inline bool) error {
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}

	// set headers
	mime := http.DetectContentType(bs)
	if inline {
		w.Header().Set(ContentDisposition, fmt.Sprintf("%s; filename=%s", contentDispositionInline, filename))
	} else {
		w.Header().Set(ContentDisposition, fmt.Sprintf("%s; filename=%s", contentDispositionAttachment, filename))
	}
	w.Header().Set(ContentType, mime)
	w.WriteHeader(status)

	_, err = w.Write(bs)
	return err
}

// file serve file as response
func (r *Render) file(w http.ResponseWriter, status int, fpath, name, contentDisposition string) error {
	var bs []byte
	var err error
	bs, err = ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(bs)

	// filename, ext, mimes
	var fn, mime, ext string
	fn, err = filepath.Abs(fpath)
	ext = filepath.Ext(fpath)
	if name != "" {
		if !strings.HasSuffix(name, ext) {
			fn = name + ext
		}
	}

	mime = http.DetectContentType(bs)

	// set headers
	w.Header().Set(ContentType, mime)
	w.Header().Set(ContentDisposition, fmt.Sprintf("%s; filename=%s", contentDisposition, fn))
	w.WriteHeader(status)

	if _, err = buf.WriteTo(w); err != nil {
		return err
	}

	_, err = w.Write(buf.Bytes())
	return err
}

// FileView serve file as response with content-disposition value inline
func (r *Render) FileView(w http.ResponseWriter, status int, fpath, name string) error {
	return r.file(w, status, fpath, name, contentDispositionInline)
}

// FileDownload serve file as response with content-disposition value attachment
func (r *Render) FileDownload(w http.ResponseWriter, status int, fpath, name string) error {
	return r.file(w, status, fpath, name, contentDispositionAttachment)
}

// parseTemplates parse all the template in the directory
func (r *Render) parseTemplates() {
	layouts, err := filepath.Glob(filepath.Join(r.opts.TemplateDir, "*"+r.opts.LayoutExtension))
	if err != nil {
		panic(fmt.Errorf("renderer: %s", err.Error()))
	}

	tpls, err := filepath.Glob(filepath.Join(r.opts.TemplateDir, "*"+r.opts.TemplateExtension))
	if err != nil {
		panic(fmt.Errorf("renderer: %s", err.Error()))
	}

	for _, tpl := range tpls {
		files := append(layouts, tpl)
		fn := filepath.Base(tpl)
		// TODO: add FuncMap and Delims
		// tmpl := template.New(fn)
		// tmpl.Delims(r.opts.LeftDelim, r.opts.RightDelim)
		// for _, fm := range r.opts.FuncMap {
		// 	tmpl.Funcs(fm)
		// }
		r.templates[fn] = template.Must(template.ParseFiles(files...))
	}
}

// parseGlob parse templates using ParseGlob
func (r *Render) parseGlob() {
	tmpl := template.New("")
	tmpl.Delims(r.opts.LeftDelim, r.opts.RightDelim)
	for _, fm := range r.opts.FuncMap {
		tmpl.Funcs(fm)
	}
	if !strings.Contains(r.opts.ParseGlobPattern, "*.") {
		log.Fatal("renderer: invalid glob pattern!")
	}
	pf := strings.Split(r.opts.ParseGlobPattern, "*")
	fPath := pf[0]
	fExt := pf[1]
	err := filepath.Walk(fPath, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, fExt) {
			_, err = tmpl.ParseFiles(path)
			if err != nil {
				log.Println(err)
			}
		}
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
	r.globTemplates = tmpl
}

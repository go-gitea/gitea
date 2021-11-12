package render

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"html/template"
	"io"
	"net/http"
)

// Engine is the generic interface for all responses.
type Engine interface {
	Render(io.Writer, interface{}) error
}

// Head defines the basic ContentType and Status fields.
type Head struct {
	ContentType string
	Status      int
}

// Data built-in renderer.
type Data struct {
	Head
}

// HTML built-in renderer.
type HTML struct {
	Head
	Name      string
	Templates *template.Template

	bp GenericBufferPool
}

// JSON built-in renderer.
type JSON struct {
	Head
	Indent        bool
	UnEscapeHTML  bool
	Prefix        []byte
	StreamingJSON bool
}

// JSONP built-in renderer.
type JSONP struct {
	Head
	Indent   bool
	Callback string
}

// Text built-in renderer.
type Text struct {
	Head
}

// XML built-in renderer.
type XML struct {
	Head
	Indent bool
	Prefix []byte
}

// Write outputs the header content.
func (h Head) Write(w http.ResponseWriter) {
	w.Header().Set(ContentType, h.ContentType)
	w.WriteHeader(h.Status)
}

// Render a data response.
func (d Data) Render(w io.Writer, v interface{}) error {
	if hw, ok := w.(http.ResponseWriter); ok {
		c := hw.Header().Get(ContentType)
		if c != "" {
			d.Head.ContentType = c
		}
		d.Head.Write(hw)
	}

	_, _ = w.Write(v.([]byte))
	return nil
}

// Render a HTML response.
func (h HTML) Render(w io.Writer, binding interface{}) error {
	var buf *bytes.Buffer
	if h.bp != nil {
		// If we have a bufferpool, allocate from it
		buf = h.bp.Get()
		defer h.bp.Put(buf)
	}

	err := h.Templates.ExecuteTemplate(buf, h.Name, binding)
	if err != nil {
		return err
	}

	if hw, ok := w.(http.ResponseWriter); ok {
		h.Head.Write(hw)
	}
	_, _ = buf.WriteTo(w)

	return nil
}

// Render a JSON response.
func (j JSON) Render(w io.Writer, v interface{}) error {
	if j.StreamingJSON {
		return j.renderStreamingJSON(w, v)
	}

	var result []byte
	var err error

	if j.Indent {
		result, err = json.MarshalIndent(v, "", "  ")
		result = append(result, '\n')
	} else {
		result, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}

	// Unescape HTML if needed.
	if j.UnEscapeHTML {
		result = bytes.ReplaceAll(result, []byte("\\u003c"), []byte("<"))
		result = bytes.ReplaceAll(result, []byte("\\u003e"), []byte(">"))
		result = bytes.ReplaceAll(result, []byte("\\u0026"), []byte("&"))
	}

	// JSON marshaled fine, write out the result.
	if hw, ok := w.(http.ResponseWriter); ok {
		j.Head.Write(hw)
	}
	if len(j.Prefix) > 0 {
		_, _ = w.Write(j.Prefix)
	}
	_, _ = w.Write(result)
	return nil
}

func (j JSON) renderStreamingJSON(w io.Writer, v interface{}) error {
	if hw, ok := w.(http.ResponseWriter); ok {
		j.Head.Write(hw)
	}
	if len(j.Prefix) > 0 {
		_, _ = w.Write(j.Prefix)
	}

	return json.NewEncoder(w).Encode(v)
}

// Render a JSONP response.
func (j JSONP) Render(w io.Writer, v interface{}) error {
	var result []byte
	var err error

	if j.Indent {
		result, err = json.MarshalIndent(v, "", "  ")
	} else {
		result, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}

	// JSON marshaled fine, write out the result.
	if hw, ok := w.(http.ResponseWriter); ok {
		j.Head.Write(hw)
	}
	_, _ = w.Write([]byte(j.Callback + "("))
	_, _ = w.Write(result)
	_, _ = w.Write([]byte(");"))

	// If indenting, append a new line.
	if j.Indent {
		_, _ = w.Write([]byte("\n"))
	}
	return nil
}

// Render a text response.
func (t Text) Render(w io.Writer, v interface{}) error {
	if hw, ok := w.(http.ResponseWriter); ok {
		c := hw.Header().Get(ContentType)
		if c != "" {
			t.Head.ContentType = c
		}
		t.Head.Write(hw)
	}

	_, _ = w.Write([]byte(v.(string)))
	return nil
}

// Render an XML response.
func (x XML) Render(w io.Writer, v interface{}) error {
	var result []byte
	var err error

	if x.Indent {
		result, err = xml.MarshalIndent(v, "", "  ")
		result = append(result, '\n')
	} else {
		result, err = xml.Marshal(v)
	}
	if err != nil {
		return err
	}

	// XML marshaled fine, write out the result.
	if hw, ok := w.(http.ResponseWriter); ok {
		x.Head.Write(hw)
	}
	if len(x.Prefix) > 0 {
		_, _ = w.Write(x.Prefix)
	}
	_, _ = w.Write(result)
	return nil
}

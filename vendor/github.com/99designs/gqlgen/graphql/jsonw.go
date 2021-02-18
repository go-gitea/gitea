package graphql

import (
	"io"
)

var nullLit = []byte(`null`)
var trueLit = []byte(`true`)
var falseLit = []byte(`false`)
var openBrace = []byte(`{`)
var closeBrace = []byte(`}`)
var openBracket = []byte(`[`)
var closeBracket = []byte(`]`)
var colon = []byte(`:`)
var comma = []byte(`,`)

var Null = &lit{nullLit}
var True = &lit{trueLit}
var False = &lit{falseLit}

type Marshaler interface {
	MarshalGQL(w io.Writer)
}

type Unmarshaler interface {
	UnmarshalGQL(v interface{}) error
}

type WriterFunc func(writer io.Writer)

func (f WriterFunc) MarshalGQL(w io.Writer) {
	f(w)
}

type Array []Marshaler

func (a Array) MarshalGQL(writer io.Writer) {
	writer.Write(openBracket)
	for i, val := range a {
		if i != 0 {
			writer.Write(comma)
		}
		val.MarshalGQL(writer)
	}
	writer.Write(closeBracket)
}

type lit struct{ b []byte }

func (l lit) MarshalGQL(w io.Writer) {
	w.Write(l.b)
}

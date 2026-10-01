// Copyright 2014 Martini Authors
// Copyright 2014 The Macaron Authors
// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package binding

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	Post struct {
		Title   string `form:"title" json:"title" binding:"Required"`
		Content string `form:"content" json:"content"`
	}

	Person struct {
		Name  string `form:"name" json:"name" binding:"Required"`
		Email string `form:"email" json:"email"`
	}

	BlogPost struct {
		Post
		ID          int                     `json:"id" binding:"Required"`
		Ignored     string                  `form:"-" json:"-"`
		Ratings     []int                   `form:"rating" json:"ratings"`
		Author      Person                  `json:"author"`
		Coauthor    *Person                 `json:"coauthor"`
		HeaderImage *multipart.FileHeader   `json:"-"`
		Pictures    []*multipart.FileHeader `form:"picture" json:"-"`
	}

	EmbedPerson struct {
		*Person
	}

	Group struct {
		Name   string    `json:"name" binding:"Required"`
		People []*Person `json:"people" binding:"MinSize(1)"`
	}

	Everything struct {
		Int        int
		Int8       int8
		Int16      int16
		Uint       uint
		Uint8      uint8
		Uint64     uint64
		Bool       bool
		BoolOn     bool
		String     string
		IntPointer *int
		Map        map[string]string
	}

	Rules struct {
		AlphaDashDot string   `binding:"AlphaDashDot"`
		MinSize      string   `binding:"MinSize(3)"`
		MinSizeSlice []string `binding:"MinSize(2)"`
		MaxSize      string   `binding:"MaxSize(1)"`
		MaxSizeSlice []string `binding:"MaxSize(1)"`
		Range        int      `binding:"Range(1,2)"`
		In           string   `binding:"In(a,b)"`
		Include      string   `binding:"Include(x)"`
		Pointer      *string  `binding:"MaxSize(1)"`
	}

	RequiredForm struct {
		String        string  `binding:"Required"`
		Int           int     `binding:"Required"`
		Bool          bool    `binding:"Required"`
		Slice         []int   `binding:"Required"`
		Pointer       *string `binding:"Required"`
		PointerToZero *string `binding:"Required"`
		Unknown       string  `binding:"Nope;MaxSize(1)"`
		StopsAtFirst  string  `binding:"Required;Nope"`
	}

	AnyString string

	Trimmed struct {
		Value   AnyString  `binding:"TrimSpace;Required"`
		Pointer *AnyString `binding:"TrimSpace;Required"`
	}

	TrimForm struct {
		Trimmed
		Nested       Trimmed
		Items        []Trimmed
		PointerItems []*Trimmed
	}
)

func TestBind(t *testing.T) {
	const formType, jsonType = "application/x-www-form-urlencoded", "application/json"
	requiredTitle := Error{FieldNames: []string{"Title"}, Classification: ErrRequired, Message: "Required"}
	unsupported := Errors{{Classification: errContentType, Message: "Unsupported Content-Type"}}
	assert.Equal(t, "Required", requiredTitle.Error())
	cases := []struct {
		name        string
		method      string
		target      string
		contentType string
		body        string
		expected    any
		errs        Errors
	}{
		{name: "form missing required", contentType: formType, body: "content=C", expected: Post{Content: "C"}, errs: Errors{requiredTitle}},
		{
			name: "form malformed", contentType: formType, body: "title=%2", expected: Post{},
			errs: Errors{{Classification: errDeserialization, Message: `invalid URL escape "%2"`}, requiredTitle},
		},
		{
			name: "form nested and embedded", contentType: formType, body: "title=T&content=C&id=1&name=N&rating=4&rating=3&-=x&ignored=x",
			expected: BlogPost{Post: Post{Title: "T", Content: "C"}, ID: 1, Ratings: []int{4, 3}, Author: Person{Name: "N"}},
		},
		{name: "query on POST", target: "/?title=T", contentType: formType, expected: Post{Title: "T"}},
		{name: "query on GET", method: http.MethodGet, target: "/?title=T&content=C", expected: Post{Title: "T", Content: "C"}},
		{name: "query on GET with content type", method: http.MethodGet, target: "/?title=T", contentType: jsonType, body: `{"title":"B"}`, expected: Post{Title: "T"}},
		{name: "query on HEAD with content type", method: http.MethodHead, target: "/?title=T", contentType: jsonType, body: `{"title":"B"}`, expected: Post{Title: "T"}},
		{name: "embedded pointer", method: http.MethodGet, target: "/?name=N&email=E", expected: EmbedPerson{&Person{Name: "N", Email: "E"}}},
		{name: "embedded pointer not bound", method: http.MethodGet, target: "/", expected: EmbedPerson{}},
		{name: "DELETE without content type", method: http.MethodDelete, target: "/?title=T", expected: Post{Title: "T"}},
		{name: "POST without content type", target: "/?title=T", expected: Post{}, errs: unsupported},
		{name: "unsupported content type", method: http.MethodPatch, contentType: "text/plain", body: "title=T", expected: Post{}, errs: unsupported},
		{
			name: "json ignores form tags", method: http.MethodPut, contentType: jsonType, body: `{"title":"T","content":"C","id":1,"rating":[1],"ratings":[4,3],"author":{"name":"N"}}`,
			expected: BlogPost{Post: Post{Title: "T", Content: "C"}, ID: 1, Ratings: []int{4, 3}, Author: Person{Name: "N"}},
		},
		{name: "json whitespace body", contentType: jsonType, body: " \n", expected: Post{}, errs: Errors{requiredTitle}},
		{
			name: "json malformed", contentType: jsonType, body: `{"title":"T"`, expected: Post{Title: "T"},
			errs: Errors{{Classification: errDeserialization, Message: "jsontext: unexpected EOF after offset 12"}},
		},
		{
			name: "json null slice element", contentType: jsonType, body: `{"name":"G","people":[null,{"name":"N"}]}`,
			expected: Group{Name: "G", People: []*Person{nil, {Name: "N"}}},
		},
		{
			name: "json slice element required", contentType: jsonType, body: `{"name":"G","people":[{"email":"E"}]}`,
			expected: Group{Name: "G", People: []*Person{{Email: "E"}}},
			errs:     Errors{{FieldNames: []string{"Name"}, Classification: ErrRequired, Message: "Required"}},
		},
		{
			name: "form type conversion", contentType: formType,
			body:     "int=-1&int8=-8&int16=&uint=1&uint8=8&uint64=64&bool=true&bool_on=on&string=s&int_pointer=7",
			expected: Everything{Int: -1, Int8: -8, Uint: 1, Uint8: 8, Uint64: 64, Bool: true, BoolOn: true, String: "s", IntPointer: new(7)},
		},
		{
			name: "form type conversion errors", contentType: formType,
			body:     "int=x&int8=128&uint=-1&uint8=256&bool=maybe&int_pointer=x&map=x",
			expected: Everything{},
			errs: Errors{
				{FieldNames: []string{"Int"}, Classification: errTypeCast, Message: "Value could not be parsed as integer"},
				{FieldNames: []string{"Int8"}, Classification: errTypeCast, Message: "Value could not be parsed as integer"},
				{FieldNames: []string{"Uint"}, Classification: errTypeCast, Message: "Value could not be parsed as unsigned integer"},
				{FieldNames: []string{"Uint8"}, Classification: errTypeCast, Message: "Value could not be parsed as unsigned integer"},
				{FieldNames: []string{"Bool"}, Classification: errTypeCast, Message: "Value could not be parsed as boolean"},
				{FieldNames: []string{"IntPointer"}, Classification: errTypeCast, Message: "Value could not be parsed as integer"},
				{FieldNames: []string{"Map"}, Classification: errDeserialization, Message: "unsupported type: map"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(cmp.Or(tc.method, http.MethodPost), cmp.Or(tc.target, "/"), strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			actual := reflect.New(reflect.TypeOf(tc.expected))
			assert.Equal(t, tc.errs, NewBinder().Bind(req, actual.Interface()))
			assert.Equal(t, tc.expected, actual.Elem().Interface())
		})
	}
}

func TestBindMultipartForm(t *testing.T) {
	readFile := func(t *testing.T, fileHeader *multipart.FileHeader) string {
		file, err := fileHeader.Open()
		require.NoError(t, err)
		defer file.Close()
		content, err := io.ReadAll(file)
		require.NoError(t, err)
		return string(content)
	}

	for _, parsedBefore := range []bool{false, true} {
		t.Run(fmt.Sprintf("parsed before %v", parsedBefore), func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			for _, field := range [][2]string{{"title", "T"}, {"id", "1"}, {"rating", "3"}, {"rating", "5"}, {"name", "N"}} {
				require.NoError(t, writer.WriteField(field[0], field[1]))
			}
			for _, file := range [][2]string{{"header_image", "header.txt"}, {"picture", "a.txt"}, {"picture", "b.txt"}} {
				fileWriter, err := writer.CreateFormFile(file[0], file[1])
				require.NoError(t, err)
				_, err = fileWriter.Write([]byte("content of " + file[1]))
				require.NoError(t, err)
			}
			require.NoError(t, writer.Close())
			req := httptest.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			if parsedBefore {
				assert.Equal(t, "T", req.FormValue("title"))
			}
			var actual BlogPost
			assert.Empty(t, NewBinder().Bind(req, &actual))
			assert.Equal(t, Post{Title: "T"}, actual.Post)
			assert.Equal(t, 1, actual.ID)
			assert.Equal(t, []int{3, 5}, actual.Ratings)
			assert.Equal(t, Person{Name: "N"}, actual.Author)
			assert.Equal(t, "content of header.txt", readFile(t, actual.HeaderImage))
			require.Len(t, actual.Pictures, 2)
			assert.Equal(t, "content of a.txt", readFile(t, actual.Pictures[0]))
			assert.Equal(t, "content of b.txt", readFile(t, actual.Pictures[1]))
		})
	}

	for name, contentType := range map[string]string{"malformed body": "multipart/form-data; boundary=x", "missing boundary": "multipart/form-data"} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("--x\r\nbroken"))
			req.Header.Set("Content-Type", contentType)
			var actual Post
			errs := NewBinder().Bind(req, &actual)
			require.Len(t, errs, 2)
			assert.Equal(t, errDeserialization, errs[0].Classification)
			assert.Equal(t, ErrRequired, errs[1].Classification)
		})
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name     string
		data     any
		expected any
		errs     Errors
	}{
		{name: "rules skip zero values", data: &Rules{}},
		{
			name: "rules with valid values",
			data: &Rules{AlphaDashDot: "a-b_c.d", MinSize: "abc", MinSizeSlice: []string{"a", "b"}, MaxSize: "é", MaxSizeSlice: []string{"a"}, Range: 2, In: "b", Include: "axb", Pointer: new("a")},
		},
		{
			name: "rules with invalid values",
			data: &Rules{AlphaDashDot: "a,b", MinSize: "ab", MinSizeSlice: []string{"a"}, MaxSize: "ab", MaxSizeSlice: []string{"a", "b"}, Range: 3, In: "c", Include: "abc", Pointer: new("ab")},
			errs: Errors{
				{FieldNames: []string{"AlphaDashDot"}, Classification: ErrAlphaDashDot, Message: "AlphaDashDot"},
				{FieldNames: []string{"MinSize"}, Classification: ErrMinSize, Message: "MinSize"},
				{FieldNames: []string{"MinSizeSlice"}, Classification: ErrMinSize, Message: "MinSize"},
				{FieldNames: []string{"MaxSize"}, Classification: ErrMaxSize, Message: "MaxSize"},
				{FieldNames: []string{"MaxSizeSlice"}, Classification: ErrMaxSize, Message: "MaxSize"},
				{FieldNames: []string{"Range"}, Classification: ErrRange, Message: "Range"},
				{FieldNames: []string{"In"}, Classification: ErrIn, Message: "In"},
				{FieldNames: []string{"Include"}, Classification: ErrInclude, Message: "Include"},
				{FieldNames: []string{"Pointer"}, Classification: ErrMaxSize, Message: "MaxSize"},
			},
		},
		{
			name: "required and unknown rules",
			data: &RequiredForm{PointerToZero: new(""), Unknown: "ab"},
			errs: Errors{
				{FieldNames: []string{"String"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"Int"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"Bool"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"Slice"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"PointerToZero"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"Unknown"}, Classification: errRule, Message: `Invalid rule: "Nope"`},
				{FieldNames: []string{"Unknown"}, Classification: ErrMaxSize, Message: "MaxSize"},
				{FieldNames: []string{"StopsAtFirst"}, Classification: ErrRequired, Message: "Required"},
			},
		},
		{
			name: "nested embedded and pointer structs",
			data: &BlogPost{Coauthor: &Person{}},
			errs: Errors{
				{FieldNames: []string{"Title"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"ID"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"Name"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"Name"}, Classification: ErrRequired, Message: "Required"},
			},
		},
		{
			name:     "TrimSpace in nested embedded and slice elements",
			data:     &TrimForm{Trimmed: Trimmed{Value: " a ", Pointer: new(AnyString(" b "))}, Nested: Trimmed{Value: " "}, Items: []Trimmed{{Value: " c "}}, PointerItems: []*Trimmed{{Value: " d ", Pointer: new(AnyString(" "))}}},
			expected: &TrimForm{Trimmed: Trimmed{Value: "a", Pointer: new(AnyString("b"))}, Nested: Trimmed{Value: ""}, Items: []Trimmed{{Value: "c"}}, PointerItems: []*Trimmed{{Value: "d", Pointer: new(AnyString(""))}}},
			errs: Errors{
				{FieldNames: []string{"Value"}, Classification: ErrRequired, Message: "Required"},
				{FieldNames: []string{"Pointer"}, Classification: ErrRequired, Message: "Required"},
			},
		},
		{
			name: "TrimSpace on non-string",
			data: &struct {
				Float float64 `binding:"TrimSpace"`
			}{Float: 1.5},
			errs: Errors{{FieldNames: []string{"Float"}, Classification: errTypeCast, Message: "TrimSpace"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.errs, NewBinder().Validate(t.Context(), tc.data))
			if tc.expected != nil {
				assert.Equal(t, tc.expected, tc.data)
			}
		})
	}
}

package gqlerror

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/vektah/gqlparser/v2/ast"
)

// Error is the standard graphql error type described in https://facebook.github.io/graphql/draft/#sec-Errors
type Error struct {
	err        error                  `json:"-"`
	Message    string                 `json:"message"`
	Path       ast.Path               `json:"path,omitempty"`
	Locations  []Location             `json:"locations,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
	Rule       string                 `json:"-"`
}

func (err *Error) SetFile(file string) {
	if file == "" {
		return
	}
	if err.Extensions == nil {
		err.Extensions = map[string]interface{}{}
	}

	err.Extensions["file"] = file
}

type Location struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

type List []*Error

func (err *Error) Error() string {
	var res bytes.Buffer
	if err == nil {
		return ""
	}
	filename, _ := err.Extensions["file"].(string)
	if filename == "" {
		filename = "input"
	}
	res.WriteString(filename)

	if len(err.Locations) > 0 {
		res.WriteByte(':')
		res.WriteString(strconv.Itoa(err.Locations[0].Line))
	}

	res.WriteString(": ")
	if ps := err.pathString(); ps != "" {
		res.WriteString(ps)
		res.WriteByte(' ')
	}

	res.WriteString(err.Message)

	return res.String()
}

func (err Error) pathString() string {
	return err.Path.String()
}

func (err Error) Unwrap() error {
	return err.err
}

func (errs List) Error() string {
	var buf bytes.Buffer
	for _, err := range errs {
		buf.WriteString(err.Error())
		buf.WriteByte('\n')
	}
	return buf.String()
}

func WrapPath(path ast.Path, err error) *Error {
	return &Error{
		err:     err,
		Message: err.Error(),
		Path:    path,
	}
}

func Errorf(message string, args ...interface{}) *Error {
	return &Error{
		Message: fmt.Sprintf(message, args...),
	}
}

func ErrorPathf(path ast.Path, message string, args ...interface{}) *Error {
	return &Error{
		Message: fmt.Sprintf(message, args...),
		Path:    path,
	}
}

func ErrorPosf(pos *ast.Position, message string, args ...interface{}) *Error {
	return ErrorLocf(
		pos.Src.Name,
		pos.Line,
		pos.Column,
		message,
		args...,
	)
}

func ErrorLocf(file string, line int, col int, message string, args ...interface{}) *Error {
	var extensions map[string]interface{}
	if file != "" {
		extensions = map[string]interface{}{"file": file}
	}
	return &Error{
		Message:    fmt.Sprintf(message, args...),
		Extensions: extensions,
		Locations: []Location{
			{Line: line, Column: col},
		},
	}
}

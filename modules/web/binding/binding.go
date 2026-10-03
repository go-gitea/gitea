// Copyright 2014 Martini Authors
// Copyright 2014 The Macaron Authors
// Copyright 2020 The Gitea Authors
// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package binding binds form, multipart and JSON request data to structs and validates them by their "binding" tags.
package binding

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"gitea.dev/modules/json"
	"gitea.dev/modules/util"
)

const (
	errContentType     = "ContentTypeError"
	errDeserialization = "DeserializationError"
	errTypeCast        = "TypeCastError"
	errRule            = "RuleError"

	ErrRequired     = "RequiredError"
	ErrAlphaDashDot = "AlphaDashDotError"
	ErrMinSize      = "MinSizeError"
	ErrMaxSize      = "MaxSizeError"
	ErrRange        = "RangeError"
	ErrIn           = "InError"
	ErrInclude      = "IncludeError"
)

const multipartMaxMemory = 10 * 1024 * 1024

type (
	Errors []Error

	Error struct {
		FieldNames     []string
		Classification string
		Message        string
	}
)

func (e Error) Error() string {
	return e.Message
}

func (e *Errors) addDeserializationError(err error) {
	*e = append(*e, Error{Classification: errDeserialization, Message: err.Error()})
}

func (e *Errors) addOptional(err *Error) {
	if err != nil {
		*e = append(*e, *err)
	}
}

func newFieldError(field reflect.StructField, classification, message string) *Error {
	return &Error{FieldNames: []string{field.Name}, Classification: classification, Message: message}
}

type ValidationField struct {
	StructField  reflect.StructField
	reflectValue reflect.Value
	ruleArgs     []string
}

func (f *ValidationField) valueAsString() string {
	return fmt.Sprint(reflect.Indirect(f.reflectValue).Interface())
}

func (f *ValidationField) ValueMustString() string {
	value := reflect.Indirect(f.reflectValue)
	if value.Kind() != reflect.String {
		panic("field value must be a string")
	}
	return value.String()
}

func (f *ValidationField) valueSize() int {
	value := reflect.Indirect(f.reflectValue)
	switch value.Kind() {
	case reflect.String:
		return utf8.RuneCountInString(value.String())
	case reflect.Slice:
		return value.Len()
	}
	panic("unsupported type: " + value.Kind().String())
}

func (f *ValidationField) assignValue(newValue any) {
	if f.reflectValue.Kind() == reflect.Pointer {
		ptr := reflect.New(f.StructField.Type.Elem())
		ptr.Elem().Set(reflect.ValueOf(newValue).Convert(ptr.Elem().Type()))
		f.reflectValue.Set(ptr)
		return
	}
	f.reflectValue.Set(reflect.ValueOf(newValue).Convert(f.reflectValue.Type()))
}

type RuleValidator func(ctx context.Context, field *ValidationField) *Error

type ruleValidatorItem struct {
	forZeroValue bool
	validatorFn  RuleValidator
}

type Binder struct {
	rules map[string]ruleValidatorItem
}

func NewBinder() *Binder {
	binder := &Binder{rules: map[string]ruleValidatorItem{}}
	binder.AddRuleNonZero("TrimSpace", func(_ context.Context, field *ValidationField) *Error {
		stringType := reflect.TypeFor[string]()
		value := reflect.Indirect(field.reflectValue)
		if !value.CanConvert(stringType) {
			return newFieldError(field.StructField, errTypeCast, "TrimSpace")
		}
		field.assignValue(strings.TrimSpace(value.Convert(stringType).String()))
		return nil
	})
	binder.rules["Required"] = ruleValidatorItem{forZeroValue: true, validatorFn: func(_ context.Context, field *ValidationField) *Error {
		// a pointer field is optional, so "Required" only applies once a value is provided
		if field.reflectValue.Kind() == reflect.Pointer && field.reflectValue.IsNil() {
			return nil
		}
		return newFieldError(field.StructField, ErrRequired, "Required")
	}}
	binder.AddRuleNonZero("AlphaDashDot", func(_ context.Context, field *ValidationField) *Error {
		if nonAlphaDashDotPattern().MatchString(field.ValueMustString()) {
			return newFieldError(field.StructField, ErrAlphaDashDot, "AlphaDashDot")
		}
		return nil
	})
	binder.AddRuleNonZero("MinSize", func(_ context.Context, field *ValidationField) *Error {
		minSize, _ := strconv.Atoi(field.ruleArgs[0])
		if field.valueSize() < minSize {
			return newFieldError(field.StructField, ErrMinSize, "MinSize")
		}
		return nil
	})
	binder.AddRuleNonZero("MaxSize", func(_ context.Context, field *ValidationField) *Error {
		maxSize, _ := strconv.Atoi(field.ruleArgs[0])
		if field.valueSize() > maxSize {
			return newFieldError(field.StructField, ErrMaxSize, "MaxSize")
		}
		return nil
	})
	binder.AddRuleNonZero("Range", func(_ context.Context, field *ValidationField) *Error {
		value, _ := strconv.Atoi(field.valueAsString())
		minValue, _ := strconv.Atoi(field.ruleArgs[0])
		maxValue, _ := strconv.Atoi(field.ruleArgs[1])
		if value < minValue || value > maxValue {
			return newFieldError(field.StructField, ErrRange, "Range")
		}
		return nil
	})
	binder.AddRuleNonZero("In", func(_ context.Context, field *ValidationField) *Error {
		if !slices.Contains(field.ruleArgs, field.valueAsString()) {
			return newFieldError(field.StructField, ErrIn, "In")
		}
		return nil
	})
	binder.AddRuleNonZero("Include", func(_ context.Context, field *ValidationField) *Error {
		if !strings.Contains(field.ValueMustString(), strings.Join(field.ruleArgs, ",")) {
			return newFieldError(field.StructField, ErrInclude, "Include")
		}
		return nil
	})
	return binder
}

func (b *Binder) AddRuleNonZero(name string, ruleValidator RuleValidator) {
	b.rules[name] = ruleValidatorItem{validatorFn: ruleValidator}
}

func (b *Binder) Bind(req *http.Request, obj any) Errors {
	ensurePointer(obj)
	contentType := req.Header.Get("Content-Type")
	if req.Method == http.MethodGet || req.Method == http.MethodHead ||
		(contentType == "" && req.Method != http.MethodPost && req.Method != http.MethodPut) {
		return b.bindForm(req, obj)
	}
	switch {
	case strings.Contains(contentType, "form-urlencoded"):
		return b.bindForm(req, obj)
	case strings.Contains(contentType, "multipart/form-data"):
		return b.bindMultipartForm(req, obj)
	case strings.Contains(contentType, "json"):
		return b.bindJSON(req, obj)
	}
	return Errors{{Classification: errContentType, Message: "Unsupported Content-Type"}}
}

func (b *Binder) bindForm(req *http.Request, formStruct any) (errs Errors) {
	if err := req.ParseForm(); err != nil {
		errs.addDeserializationError(err)
	}
	errs = mapForm(reflect.ValueOf(formStruct), req.Form, nil, errs)
	return append(errs, b.Validate(req.Context(), formStruct)...)
}

func (b *Binder) bindMultipartForm(req *http.Request, formStruct any) (errs Errors) {
	if err := req.ParseMultipartForm(multipartMaxMemory); err != nil {
		errs.addDeserializationError(err)
	}
	if req.MultipartForm != nil {
		errs = mapForm(reflect.ValueOf(formStruct), req.MultipartForm.Value, req.MultipartForm.File, errs)
	}
	return append(errs, b.Validate(req.Context(), formStruct)...)
}

func (b *Binder) bindJSON(req *http.Request, jsonStruct any) (errs Errors) {
	err := json.NewDecoder(req.Body).Decode(jsonStruct)
	if err != nil && !errors.Is(err, io.EOF) { // an empty body binds nothing
		errs.addDeserializationError(err)
	}
	return append(errs, b.Validate(req.Context(), jsonStruct)...)
}

func (b *Binder) Validate(ctx context.Context, obj any) Errors {
	ensurePointer(obj)
	return b.validateStruct(ctx, nil, reflect.ValueOf(obj).Elem())
}

func (b *Binder) validateStruct(ctx context.Context, errs Errors, structValue reflect.Value) Errors {
	structType := structValue.Type()
	for i := range structType.NumField() {
		field := structType.Field(i)
		fieldValue := structValue.Field(i)
		if field.Tag.Get("form") == "-" || !fieldValue.CanInterface() {
			continue
		}
		if field.Type.Kind() == reflect.Struct ||
			(field.Type.Kind() == reflect.Pointer && !fieldValue.IsNil() && field.Type.Elem().Kind() == reflect.Struct) {
			errs = b.validateStruct(ctx, errs, reflect.Indirect(fieldValue))
			continue
		}
		errs = b.validateField(ctx, errs, &ValidationField{StructField: field, reflectValue: fieldValue})
	}
	return errs
}

func (b *Binder) validateField(ctx context.Context, errs Errors, field *ValidationField) Errors {
	if field.reflectValue.Kind() == reflect.Slice {
		for i := range field.reflectValue.Len() {
			if elem := reflect.Indirect(field.reflectValue.Index(i)); elem.Kind() == reflect.Struct {
				errs = b.validateStruct(ctx, errs, elem)
			}
		}
	}

	for rule := range strings.SplitSeq(field.StructField.Tag.Get("binding"), ";") {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		ruleName, ruleArgs, _ := strings.Cut(rule, "(")
		field.ruleArgs = nil
		if ruleArgs != "" {
			field.ruleArgs = strings.Split(strings.TrimSuffix(ruleArgs, ")"), ",")
		}
		item, ok := b.rules[ruleName]
		if !ok {
			panic(fmt.Sprintf("Invalid binding rule: %q", ruleName))
		}
		value := reflect.Indirect(field.reflectValue)
		if item.forZeroValue != (!value.IsValid() || value.IsZero()) {
			continue
		}
		if err := item.validatorFn(ctx, field); err != nil {
			return append(errs, *err)
		}
	}
	return errs
}

var nonAlphaDashDotPattern = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`[^\w-.]`)
})

func mapForm(formStruct reflect.Value, form map[string][]string, formFiles map[string][]*multipart.FileHeader, errs Errors) Errors {
	formStruct = reflect.Indirect(formStruct)
	structType := formStruct.Type()

	for fieldIdx := range structType.NumField() {
		typeField := structType.Field(fieldIdx)
		fieldValue := formStruct.Field(fieldIdx)

		if typeField.Type.Kind() == reflect.Pointer && typeField.Anonymous {
			fieldValue.Set(reflect.New(typeField.Type.Elem()))
			errs = mapForm(fieldValue.Elem(), form, formFiles, errs)
			if fieldValue.Elem().IsZero() {
				fieldValue.SetZero()
			}
		} else if typeField.Type.Kind() == reflect.Struct {
			errs = mapForm(fieldValue, form, formFiles, errs)
		}

		inputFieldName := typeField.Tag.Get("form")
		if inputFieldName == "-" || !typeField.IsExported() {
			continue
		}
		if inputFieldName == "" {
			inputFieldName = util.ToSnakeCase(typeField.Name)
		}

		if inputValues, exists := form[inputFieldName]; exists {
			if fieldValue.Kind() == reflect.Slice && len(inputValues) > 0 {
				slice := reflect.MakeSlice(fieldValue.Type(), len(inputValues), len(inputValues))
				for elemIdx, inputValue := range inputValues {
					errs.addOptional(setWithProperType(typeField, inputValue, slice.Index(elemIdx)))
				}
				fieldValue.Set(slice)
			} else {
				errs.addOptional(setWithProperType(typeField, inputValues[0], fieldValue))
			}
			continue
		}

		inputFiles, exists := formFiles[inputFieldName]
		if !exists {
			continue
		}
		fileHeaderType := reflect.TypeFor[*multipart.FileHeader]()
		if fieldValue.Kind() == reflect.Slice && len(inputFiles) > 0 && fieldValue.Type().Elem() == fileHeaderType {
			fieldValue.Set(reflect.ValueOf(slices.Clone(inputFiles)))
		} else if fieldValue.Type() == fileHeaderType {
			fieldValue.Set(reflect.ValueOf(inputFiles[0]))
		}
	}
	return errs
}

func setWithProperType(structField reflect.StructField, val string, fieldValue reflect.Value) *Error {
	switch fieldValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(cmp.Or(val, "0"), 10, fieldValue.Type().Bits())
		if err != nil {
			return newFieldError(structField, errTypeCast, "Value could not be parsed as integer")
		}
		fieldValue.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(cmp.Or(val, "0"), 10, fieldValue.Type().Bits())
		if err != nil {
			return newFieldError(structField, errTypeCast, "Value could not be parsed as unsigned integer")
		}
		fieldValue.SetUint(uintVal)
	case reflect.Bool:
		if val == "on" {
			fieldValue.SetBool(true)
			break
		}
		boolVal, err := strconv.ParseBool(cmp.Or(val, "false"))
		if err != nil {
			return newFieldError(structField, errTypeCast, "Value could not be parsed as boolean")
		}
		fieldValue.SetBool(boolVal)
	case reflect.String:
		fieldValue.SetString(val)
	case reflect.Pointer:
		newValue := reflect.New(fieldValue.Type().Elem())
		if err := setWithProperType(structField, val, newValue.Elem()); err != nil {
			return err
		}
		fieldValue.Set(newValue)
	default:
		return newFieldError(structField, errDeserialization, "unsupported type: "+fieldValue.Kind().String())
	}
	return nil
}

func ensurePointer(obj any) {
	if reflect.TypeOf(obj).Kind() != reflect.Pointer {
		panic("Pointers are only accepted as binding models")
	}
}

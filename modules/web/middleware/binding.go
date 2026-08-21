// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"reflect"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"

	"gitea.com/go-chi/binding" //nolint:depguard // this package wraps it
)

type (
	ValidateContext      = structs.ValidateContext
	FormDefaultValidator = structs.FormDefaultValidator
)

type Form interface {
	Validate(ctx *ValidateContext, errs validation.BindingErrors) validation.BindingErrors
}

// BindFormAny binds the request to the form of type T and returns the pointer to the form and any binding errors.
// Only the rules defined in the struct field's "binding" tag are applied.
// It can bind to any struct, doesn't call the struct's "Form.Validate" interface.
func BindFormAny[T any](req *http.Request, binder *binding.Binder, _ T) (ret *T, _ validation.BindingErrors) {
	typ := reflect.TypeFor[T]()
	if typ.Kind() != reflect.Struct {
		panic("BindFormAny: template type must be a struct and the function returns its pointer")
	}
	form := new(T)
	errs := binder.Bind(req, form)
	return form, errs
}

// BindFormValidate binds the request to the form of type T which must be a pointer implementing Form interface
// After binding, the Form.Validate is also called so we can do more validation checks
func BindFormValidate[T Form](req *http.Request, binder *binding.Binder) (ret T, _ validation.BindingErrors) {
	locale := req.Context().Value(translation.ContextKey).(translation.Locale) //nolint:forcetypeassert // must exist
	ptrType := reflect.TypeFor[T]()
	structType := ptrType.Elem()
	ptrVal := reflect.New(structType)
	form := ptrVal.Interface().(Form) //nolint:forcetypeassert // must implement Form
	errs := binder.Bind(req, form)
	errs = form.Validate(&ValidateContext{Locale: locale}, errs)
	return form.(T), errs //nolint:forcetypeassert // must be type T
}

// AssignForm assign form values back to the template data, the template variable names are in "snake_case"
func AssignForm(form any, data map[string]any) {
	typ := reflect.TypeOf(form)
	val := reflect.ValueOf(form)

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
		val = val.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		fieldName := field.Tag.Get("form")
		// Allow ignored fields in the struct
		if fieldName == "-" {
			continue
		} else if len(fieldName) == 0 {
			fieldName = util.ToSnakeCase(field.Name)
		}

		data[fieldName] = val.Field(i).Interface()
	}
}

func getRuleBody(field reflect.StructField, ruleName string) string {
	prefix := ruleName + "("
	for rule := range strings.SplitSeq(field.Tag.Get("binding"), ";") {
		if strings.HasPrefix(rule, prefix) {
			return rule[len(prefix) : len(rule)-1]
		}
	}
	return ""
}

func AddValidationError(errs validation.BindingErrors, fieldName, errorMsg string) validation.BindingErrors {
	errs.Add([]string{fieldName}, validation.ErrCustomMessage, errorMsg)
	return errs
}

func getFieldDisplayNameForMessage(f any, l translation.Locale, fieldNames []string) (field reflect.StructField, ok bool, displayName string) {
	if len(fieldNames) == 0 {
		return field, false, ""
	}
	typ := reflect.TypeOf(f)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	fieldName := fieldNames[0]
	field, fieldExists := typ.FieldByName(fieldName)
	if !fieldExists {
		for tryField := range typ.Fields() {
			if util.ToSnakeCase(tryField.Name) == fieldName || tryField.Tag.Get("form") == fieldName {
				field, fieldExists = tryField, true
			}
		}
		if !fieldExists {
			return field, false, ""
		}
	}

	if field.Tag.Get("form") == "-" {
		return field, false, ""
	}

	trKeyFallback := "form." + field.Name
	trKey := util.IfZero(field.Tag.Get("locale"), trKeyFallback)
	if l.HasKey(trKey) {
		displayName = l.TrString(trKey)
	} else {
		displayName = field.Name
	}
	return field, true, displayName
}

func BuildValidationErrorForUser(f any, l translation.Locale, bindingErrs validation.BindingErrors) (errorMessage, errorFieldName string, fieldNames []string) {
	if bindingErrs.Len() == 0 {
		return "", "", nil
	}
	bindingErr := bindingErrs[0]
	fieldNames, classification, bindingErrMsg := bindingErr.FieldNames, bindingErr.Classification, bindingErr.Message
	field, ok, fieldDisplayName := getFieldDisplayNameForMessage(f, l, fieldNames)
	if !ok {
		return l.TrString("error.occurred"), "", fieldNames
	}

	errorFieldName = field.Name
	switch classification {
	case binding.ERR_REQUIRED:
		errorMessage = l.TrString("form.require_error", fieldDisplayName)
	case binding.ERR_ALPHA_DASH:
		errorMessage = l.TrString("form.alpha_dash_error", fieldDisplayName)
	case binding.ERR_ALPHA_DASH_DOT:
		errorMessage = l.TrString("form.alpha_dash_dot_error", fieldDisplayName)
	case binding.ERR_MIN_SIZE:
		errorMessage = l.TrString("form.min_size_error", fieldDisplayName, getRuleBody(field, "MinSize"))
	case binding.ERR_MAX_SIZE:
		errorMessage = l.TrString("form.max_size_error", fieldDisplayName, getRuleBody(field, "MaxSize"))
	case binding.ERR_RANGE:
		rangeMin, rangeMax, _ := strings.Cut(getRuleBody(field, "Range"), ",")
		errorMessage = l.TrString("form.range_error", fieldDisplayName, rangeMin, rangeMax)
	case binding.ERR_EMAIL:
		errorMessage = l.TrString("form.email_error", fieldDisplayName)
	case binding.ERR_URL:
		errorMessage = l.TrString("form.url_error", fieldDisplayName)
	case binding.ERR_IN:
		ruleBody := getRuleBody(field, "In")
		if strings.HasPrefix(ruleBody, ",") {
			ruleBody = "(empty)" + ruleBody
		}
		errorMessage = l.TrString("form.in_error", fieldDisplayName, ruleBody)
	case binding.ERR_INCLUDE:
		errorMessage = l.TrString("form.include_error", fieldDisplayName, getRuleBody(field, "Include"))

	case validation.ErrCustomMessage:
		errorMessage = bindingErrMsg
	case validation.ErrGitRefName:
		errorMessage = l.TrString("form.git_ref_name_error", fieldDisplayName)
	case validation.ErrGlobPattern:
		errorMessage = l.TrString("form.glob_pattern_error", fieldDisplayName, bindingErrMsg)
	case validation.ErrRegexPattern:
		errorMessage = l.TrString("form.regex_pattern_error", fieldDisplayName, bindingErrMsg)
	case validation.ErrUsername:
		errorMessage = l.TrString("form.username_error", fieldDisplayName)
	case validation.ErrInvalidGroupTeamMap:
		errorMessage = l.TrString("form.invalid_group_team_map_error", fieldDisplayName, bindingErrMsg)
	case validation.ErrInvalidBadgeSlug:
		errorMessage = l.TrString("form.invalid_slug_error", fieldDisplayName)
	default:
		setting.PanicInDevOrTesting("unknown binding error classification for field %T.%s: %v, err: %s", f, errorFieldName, classification, bindingErrMsg)
		var msg string
		if classification != "" && bindingErrMsg != "" {
			msg = classification + ": " + bindingErrMsg
		} else {
			msg = util.IfZero(bindingErrMsg, classification)
			if msg == "" {
				setting.PanicInDevOrTesting("no error message for binding error: %v", bindingErr)
			}
			msg = util.IfZero(msg, "unknown error")
		}
		errorMessage = l.TrString("form.field_invalid_message", fieldDisplayName, msg)
	}
	return errorMessage, errorFieldName, fieldNames
}

// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"gitea.com/go-chi/binding"
)

// Form form binding interface
type Form interface {
	binding.Validator
}

func init() {
	binding.SetNameMapper(util.ToSnakeCase)
}

// AssignForm assign form values back to the template data.
func AssignForm(form any, data map[string]any) {
	typ := reflect.TypeOf(form)
	val := reflect.ValueOf(form)

	for typ.Kind() == reflect.Ptr {
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

func getRuleBody(field reflect.StructField, prefix string) string {
	for rule := range strings.SplitSeq(field.Tag.Get("binding"), ";") {
		if strings.HasPrefix(rule, prefix) {
			return rule[len(prefix) : len(rule)-1]
		}
	}
	return ""
}

// GetSize get size int form tag
func GetSize(field reflect.StructField) string {
	return getRuleBody(field, "Size(")
}

// GetMinSize get minimal size in form tag
func GetMinSize(field reflect.StructField) string {
	return getRuleBody(field, "MinSize(")
}

// GetMaxSize get max size in form tag
func GetMaxSize(field reflect.StructField) string {
	return getRuleBody(field, "MaxSize(")
}

// GetInclude get include in form tag
func GetInclude(field reflect.StructField) string {
	return getRuleBody(field, "Include(")
}

func ReportValidationError(errs binding.Errors, data map[string]any, fieldName, classification, errorMsg string) binding.Errors {
	errs.Add([]string{fieldName}, classification, errorMsg)

	data["HasError"] = true
	data["ErrorMsg"] = fieldName + ": " + errorMsg
	data["Err_"+fieldName] = true
	// there is already a reported validation error, so no need to generate default error messages in Validate()
	data["HasErrorFormValidation"] = true
	return errs
}

func Validate(errs binding.Errors, data map[string]any, f Form, l translation.Locale) binding.Errors {
	// try to restore the form's values as much as possible,
	// especially for RenderWithErrDeprecated to re-render the form with errors
	AssignForm(f, data)

	if errs.Len() == 0 || data["HasErrorFormValidation"] == true {
		return errs
	}

	// if HasError=true, then must set default error message
	// because still a lot of places use `ctx.Data["ErrorMsg"].(string)` even if the error fields can't be found
	data["HasError"] = true
	data["ErrorMsg"] = l.TrString("form.unknown_error")

	typ := reflect.TypeOf(f)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	field, fieldExists := typ.FieldByName(errs[0].FieldNames[0])
	if !fieldExists {
		return errs
	}

	if field.Tag.Get("form") == "-" {
		return errs
	}

	data["Err_"+field.Name] = true

	trName := field.Tag.Get("locale")
	if len(trName) == 0 {
		trName = l.TrString("form." + field.Name)
	} else {
		trName = l.TrString(trName)
	}

	switch errs[0].Classification {
	case binding.ERR_REQUIRED:
		data["ErrorMsg"] = trName + l.TrString("form.require_error")
	case binding.ERR_ALPHA_DASH:
		data["ErrorMsg"] = trName + l.TrString("form.alpha_dash_error")
	case binding.ERR_ALPHA_DASH_DOT:
		data["ErrorMsg"] = trName + l.TrString("form.alpha_dash_dot_error")
	case validation.ErrGitRefName:
		data["ErrorMsg"] = trName + l.TrString("form.git_ref_name_error")
	case binding.ERR_SIZE:
		data["ErrorMsg"] = trName + l.TrString("form.size_error", GetSize(field))
	case binding.ERR_MIN_SIZE:
		data["ErrorMsg"] = trName + l.TrString("form.min_size_error", GetMinSize(field))
	case binding.ERR_MAX_SIZE:
		data["ErrorMsg"] = trName + l.TrString("form.max_size_error", GetMaxSize(field))
	case binding.ERR_EMAIL:
		data["ErrorMsg"] = trName + l.TrString("form.email_error")
	case binding.ERR_URL:
		data["ErrorMsg"] = trName + l.TrString("form.url_error", errs[0].Message)
	case binding.ERR_INCLUDE:
		data["ErrorMsg"] = trName + l.TrString("form.include_error", GetInclude(field))
	case validation.ErrGlobPattern:
		data["ErrorMsg"] = trName + l.TrString("form.glob_pattern_error", errs[0].Message)
	case validation.ErrRegexPattern:
		data["ErrorMsg"] = trName + l.TrString("form.regex_pattern_error", errs[0].Message)
	case validation.ErrUsername:
		data["ErrorMsg"] = trName + l.TrString("form.username_error")
	case validation.ErrInvalidGroupTeamMap:
		data["ErrorMsg"] = trName + l.TrString("form.invalid_group_team_map_error", errs[0].Message)
	case validation.ErrInvalidBadgeSlug:
		data["ErrorMsg"] = trName + l.TrString("form.invalid_slug_error")
	default:
		msg := errs[0].Classification
		if msg != "" && errs[0].Message != "" {
			msg += ": "
		}

		msg += errs[0].Message
		if msg == "" {
			msg = l.TrString("form.unknown_error")
		}
		data["ErrorMsg"] = trName + ": " + msg
	}

	return errs
}

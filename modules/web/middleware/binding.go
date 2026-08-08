// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"net/http"
	"reflect"
	"strings"

	"gitea.dev/modules/json"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
	"gitea.dev/modules/validation"

	"gitea.com/go-chi/binding"
)

// ValidateContext is a special context for form validation middleware. It may be different from other contexts.
type ValidateContext struct {
	Locale translation.Locale
	Data   reqctx.ContextData
	Req    *http.Request
	Resp   http.ResponseWriter
}

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

func AddValidationError(errs binding.Errors, fieldName, errorMsg string) binding.Errors {
	errs.Add([]string{fieldName}, validation.ErrCustomMessage, errorMsg)
	return errs
}

func getFieldDisplayNameForMessage(f Form, l translation.Locale, fieldNames []string) (field reflect.StructField, ok bool, displayName string) {
	if len(fieldNames) == 0 {
		return field, false, ""
	}
	typ := reflect.TypeOf(f)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	field, fieldExists := typ.FieldByName(fieldNames[0])
	if !fieldExists {
		return field, false, ""
	}

	if field.Tag.Get("form") == "-" {
		return field, false, ""
	}

	trKeyFallback := "form." + field.Name
	trKey := util.IfZero(field.Tag.Get("locale"), trKeyFallback)
	displayName = l.TrString(trKey)
	if displayName == trKeyFallback {
		displayName = field.Name
	}
	return field, true, displayName
}

func buildValidationErrorForUser(f Form, l translation.Locale, bindingErrs binding.Errors) (errorMessage, errorFieldName string, fieldNames []string) {
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
		setting.PanicInDevOrTesting("unknown binding error classification: %v", classification)
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

func Validate(ctx *ValidateContext, errs binding.Errors, f Form) binding.Errors {
	// try to restore the form's values as much as possible,
	// especially for RenderWithErrDeprecated to re-render the form with errors
	AssignForm(f, ctx.Data)

	errorMessage, errorFieldName, fieldNames := buildValidationErrorForUser(f, ctx.Locale, errs)
	if errorMessage == "" {
		return errs
	}

	if ctx.Req.Header.Get("X-Gitea-Fetch-Action") != "" {
		ctx.Resp.Header().Set("Content-Type", "application/json")
		_ = json.MarshalWrite(ctx.Resp, map[string]any{
			"errorMessage": errorMessage,
			"errorFields":  fieldNames,
		})
		return errs
	}

	// legacy template error display
	ctx.Data["HasError"] = true
	ctx.Data["ErrorMsg"] = errorMessage
	if errorFieldName != "" {
		ctx.Data["Err_"+errorFieldName] = true
	}
	return errs
}

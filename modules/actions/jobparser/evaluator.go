// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gitea.com/gitea/runner/act/exprparser"
	"go.yaml.in/yaml/v4"
)

// ExpressionEvaluator is copied from runner.expressionEvaluator,
// to avoid unnecessary dependencies
type ExpressionEvaluator struct {
	interpreter exprparser.Interpreter
}

func NewExpressionEvaluator(interpreter exprparser.Interpreter) *ExpressionEvaluator {
	return &ExpressionEvaluator{interpreter: interpreter}
}

func (ee ExpressionEvaluator) evaluateScalarYamlNode(node *yaml.Node) error {
	var in string
	if err := node.Decode(&in); err != nil {
		return err
	}
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return nil
	}
	res, err := ee.evaluateScalar(in)
	if err != nil {
		return err
	}
	return node.Encode(res)
}

// GitHub has this undocumented feature to merge maps, called insert directive
var insertDirective = regexp.MustCompile(`\${{\s*insert\s*}}`)

func (ee ExpressionEvaluator) evaluateMappingYamlNode(node *yaml.Node) error {
	for i := 0; i < len(node.Content)/2; {
		k := node.Content[i*2]
		v := node.Content[i*2+1]
		if err := ee.EvaluateYamlNode(v); err != nil {
			return err
		}
		var sk string
		// Merge the nested map of the insert directive
		if k.Decode(&sk) == nil && insertDirective.MatchString(sk) {
			node.Content = append(append(node.Content[:i*2], v.Content...), node.Content[(i+1)*2:]...)
			i += len(v.Content) / 2
		} else {
			if err := ee.EvaluateYamlNode(k); err != nil {
				return err
			}
			i++
		}
	}
	return nil
}

func (ee ExpressionEvaluator) evaluateSequenceYamlNode(node *yaml.Node) error {
	for i := 0; i < len(node.Content); {
		v := node.Content[i]
		// Preserve nested sequences
		wasseq := v.Kind == yaml.SequenceNode
		if err := ee.EvaluateYamlNode(v); err != nil {
			return err
		}
		// GitHub has this undocumented feature to merge sequences / arrays
		// We have a nested sequence via evaluation, merge the arrays
		if v.Kind == yaml.SequenceNode && !wasseq {
			node.Content = append(append(node.Content[:i], v.Content...), node.Content[i+1:]...)
			i += len(v.Content)
		} else {
			i++
		}
	}
	return nil
}

func (ee ExpressionEvaluator) EvaluateYamlNode(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return ee.evaluateScalarYamlNode(node)
	case yaml.MappingNode:
		return ee.evaluateMappingYamlNode(node)
	case yaml.SequenceNode:
		return ee.evaluateSequenceYamlNode(node)
	default:
		return nil
	}
}

func (ee ExpressionEvaluator) Interpolate(in string) string {
	out, err := ee.interpolate(in)
	if err != nil {
		return ""
	}
	return out
}

// interpolate evaluates every part on its own, so a malformed one cannot restructure its neighbours
func (ee ExpressionEvaluator) interpolate(in string) (string, error) {
	parts, err := splitSubExpressions(in)
	if err != nil {
		return "", err
	}
	if len(parts) == 1 && !parts[0].isExpr {
		return in, nil
	}
	var out strings.Builder
	out.Grow(len(in))
	for _, part := range parts {
		if !part.isExpr {
			out.WriteString(part.text)
			continue
		}
		evaluated, err := ee.interpreter.Evaluate(part.text, exprparser.DefaultStatusCheckNone)
		if err != nil {
			return "", err
		}
		out.WriteString(coerceToString(evaluated))
	}
	return out.String(), nil
}

// evaluateScalar keeps the type of a lone expression, so `${{ fromJSON('[1,2]') }}` stays an array
func (ee ExpressionEvaluator) evaluateScalar(in string) (any, error) {
	parts, err := splitSubExpressions(in)
	if err != nil {
		return nil, err
	}
	if len(parts) == 1 && parts[0].isExpr {
		return ee.interpreter.Evaluate(parts[0].text, exprparser.DefaultStatusCheckNone)
	}
	return ee.interpolate(in)
}

// evaluateCondition evaluates an `if:`, an expression even without `${{ }}`. Mixed content
// interpolates to a string, so the success() default applies to it separately.
func (ee ExpressionEvaluator) evaluateCondition(in string) (bool, error) {
	parts, err := splitSubExpressions(in)
	if err != nil {
		return false, err
	}
	if len(parts) == 1 {
		evaluated, err := ee.interpreter.Evaluate(parts[0].text, exprparser.DefaultStatusCheckSuccess)
		if err != nil {
			return false, err
		}
		return exprparser.IsTruthy(evaluated), nil
	}

	if !expressionCallsStatusFunction(in) {
		status, err := ee.interpreter.Evaluate("success()", exprparser.DefaultStatusCheckNone)
		if err != nil {
			return false, err
		}
		if !exprparser.IsTruthy(status) {
			return false, nil
		}
	}
	interpolated, err := ee.interpolate(in)
	if err != nil {
		return false, err
	}
	return exprparser.IsTruthy(interpolated), nil
}

// coerceToString converts an evaluated expression value to a string the way GitHub does,
// see https://docs.github.com/en/actions/reference/workflows-and-actions/expressions#operators
// An already reflected value is accepted as-is, since Interface() would panic on an invalid one.
func coerceToString(v any) string {
	value, ok := v.(reflect.Value)
	if !ok {
		value = reflect.ValueOf(v)
	}

	switch value.Kind() {
	case reflect.Invalid:
		return ""

	case reflect.Bool:
		return strconv.FormatBool(value.Bool())

	case reflect.String:
		return value.String()

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(value.Uint(), 10)

	case reflect.Float32, reflect.Float64:
		if math.IsInf(value.Float(), 1) {
			return "Infinity"
		} else if math.IsInf(value.Float(), -1) {
			return "-Infinity"
		}
		return fmt.Sprintf("%.15G", value.Float())

	case reflect.Slice, reflect.Array:
		return "Array"

	// contexts such as `github` are pointers to structs, so they stringify as objects too
	case reflect.Map, reflect.Struct:
		return "Object"

	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return ""
		}
		return coerceToString(value.Elem())
	}

	return fmt.Sprintf("%v", value)
}

type exprPart struct {
	text   string
	isExpr bool
}

// splitSubExpressions splits in the way GitHub's template reader does, leaving a value without a
// complete expression literal.
func splitSubExpressions(in string) ([]exprPart, error) {
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return []exprPart{{text: in}}, nil
	}

	parts := make([]exprPart, 0, 2*strings.Count(in, "${{")+1)
	for {
		start := strings.Index(in, "${{")
		if start < 0 {
			if in != "" {
				parts = append(parts, exprPart{text: in})
			}
			return parts, nil
		}
		if start > 0 {
			parts = append(parts, exprPart{text: in[:start]})
		}
		rest := in[start+len("${{"):]
		end := indexExprEnd(rest)
		if end < 0 {
			return nil, errors.New("unclosed expression")
		}
		parts = append(parts, exprPart{text: strings.TrimSpace(rest[:end]), isExpr: true})
		in = rest[end+len("}}"):]
	}
}

// indexExprEnd returns the offset of the `}}` ending an expression, or -1. A quote toggles string
// state, so a `}}` inside a string does not end it.
func indexExprEnd(in string) int {
	inString := false
	for i := range len(in) {
		switch {
		case in[i] == '\'':
			inString = !inString
		case !inString && in[i] == '}' && i+1 < len(in) && in[i+1] == '}':
			return i
		}
	}
	return -1
}
